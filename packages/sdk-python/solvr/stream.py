"""An open room stream (server-sent events), read with next() or a for loop."""

import codecs
import json
from types import TracebackType
from typing import Any, Iterator, List, Optional, Type

import requests

from ._decode import decode
from .room_types import RoomStreamEvent, RoomStreamFrame, RoomStreamMessage
from .types import SolvrError

# The events that end a stream; their data is {code, message}.
STREAM_ENDS = frozenset({"access_revoked", "credential_rotated"})


class RoomStream:
    """
    An open room stream. Read events with next() or iterate it; close it with
    close() or a with block.

    Example:
        >>> with room.stream_room("parser-build") as stream:
        ...     for event in stream:
        ...         if event.message:
        ...             print(event.message.content)
    """

    def __init__(self, response: requests.Response, last_event_id: str = ""):
        self._response = response
        # chunk_size None: each chunk as it arrives, so an event is read when it is sent.
        self._chunks = response.iter_content(chunk_size=None)
        self._decoder = codecs.getincrementaldecoder("utf-8")()
        self._buffer = ""
        self._last_id = last_event_id
        self._closed = False

    @property
    def last_event_id(self) -> str:
        """The id of the last event received with one (or the one the stream resumed
        from): reconnect with it as last_event_id to replay what was missed."""
        return self._last_id

    def next(self) -> Optional[RoomStreamEvent]:
        """
        Reads the next event, skipping heartbeats. Answers None once the stream is
        closed (reconnect with last_event_id). A stream the server ended because
        the caller's access did raises the SolvrError of that code, status 0
        (CREDENTIAL_ROTATED: handshake again; ACCESS_REVOKED).
        """
        event_id = ""
        name = ""
        has_id = False
        data: List[str] = []
        while True:
            line = self._read_line()
            if line is None:
                return None
            if line:
                if line.startswith(":"):
                    continue
                field, _, value = line.partition(":")
                if value.startswith(" "):
                    value = value[1:]
                if field == "id":
                    event_id = value
                    has_id = True
                elif field == "event":
                    name = value
                elif field == "data":
                    data.append(value)
                continue
            if not data:
                event_id = ""
                name = ""
                has_id = False
                continue
            if has_id:
                self._last_id = event_id
            return self._dispatch(event_id, name or "message", "\n".join(data))

    def close(self) -> None:
        """Closes the stream."""
        if self._closed:
            return
        self._closed = True
        self._response.close()

    def __iter__(self) -> Iterator[RoomStreamEvent]:
        return self

    def __next__(self) -> RoomStreamEvent:
        event = self.next()
        if event is None:
            raise StopIteration
        return event

    def __enter__(self) -> "RoomStream":
        return self

    def __exit__(
        self,
        exc_type: Optional[Type[BaseException]],
        exc: Optional[BaseException],
        traceback: Optional[TracebackType],
    ) -> None:
        self.close()

    def _dispatch(self, event_id: str, name: str, data: str) -> RoomStreamEvent:
        if name in STREAM_ENDS:
            end: Any = None
            try:
                end = json.loads(data)
            except ValueError:
                pass
            if not isinstance(end, dict) or not end.get("code"):
                raise ValueError(f"the stream ended with {name}: {data}")
            self.close()
            raise SolvrError(end.get("message") or name, 0, end["code"], None, end.get("request_id"))
        try:
            raw = json.loads(data)
        except ValueError as error:
            raise ValueError(f"failed to decode the {name} frame: {error}") from error
        if not isinstance(raw, dict):
            raise ValueError(f"failed to decode the {name} frame: {data}")
        frame = decode(RoomStreamFrame, raw)
        payload = raw.get("payload")
        message = None
        if frame.type == "message" and isinstance(payload, dict):
            message = decode(RoomStreamMessage, payload)
        return RoomStreamEvent(id=event_id, event=name, frame=frame, message=message)

    def _read_line(self) -> Optional[str]:
        """The next line without its end of line, or None at the end of the stream."""
        while True:
            # Closed by close() or by the server (whose remaining buffer holds no whole line).
            if self._closed:
                return None
            end = self._buffer.find("\n")
            if end >= 0:
                line = self._buffer[:end]
                self._buffer = self._buffer[end + 1:]
                return line[:-1] if line.endswith("\r") else line
            try:
                chunk = next(self._chunks)
            except StopIteration:
                self.close()
                return None
            except requests.exceptions.RequestException:
                if self._closed:
                    return None
                raise
            self._buffer += self._decoder.decode(chunk)
