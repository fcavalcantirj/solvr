"""Tests for rooms, room tokens, the room stream, reply edits, and search sorting."""

import json
import threading
import time
from http.server import BaseHTTPRequestHandler

import pytest
import responses

from solvr import RoomStream, Solvr, SolvrError

from .local_server import end_stream, serve, start_stream, write_chunk

API_KEY = "solvr_sk_test_key"
ROOM_TOKEN = "solvr_rt_test_token"
BASE_URL = "https://api.solvr.dev"

ENTRY = {
    "id": 7,
    "room_id": "room_1",
    "sequence": 3,
    "kind": "message",
    "author_type": "agent",
    "author_id": "agent_1",
    "actor_label": "planner",
    "body": "Plan: build the parser.",
    "content_type": "text",
    "extension": {},
    "created_at": "2026-10-01T00:00:00Z",
}

MESSAGE = {
    "id": 7,
    "room_id": "room_1",
    "author_type": "agent",
    "author_id": "agent_1",
    "agent_name": "planner",
    "content": "Plan: build the parser.",
    "content_type": "text",
    "metadata": {},
    "sequence_num": 3,
    "created_at": "2026-10-01T00:00:00Z",
}

MESSAGE_FRAME = {
    "id": 7,
    "sequence": 3,
    "type": "message",
    "room_id": "room_1",
    "agent_name": "planner",
    "payload": MESSAGE,
    "timestamp": "2026-10-01T00:00:00Z",
}


def sse(*frames: str) -> bytes:
    return "".join(frames).encode("utf-8")


class TestCredentials:
    """The credential each client presents."""

    @responses.activate
    def test_anonymous_client_sends_no_authorization(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/posts/post_1", json={"data": {
            "id": "post_1", "type": "post", "title": "T", "description": "D"}})

        Solvr(api_key=None).get_post("post_1")

        assert "Authorization" not in responses.calls[0].request.headers

    @responses.activate
    def test_with_room_token_answers_a_copy_presenting_the_token(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/rooms/room-a/entries", json={"data": [], "meta": {}})
        responses.add(responses.GET, f"{BASE_URL}/v1/posts/post_1", json={"data": {
            "id": "post_1", "type": "post", "title": "T", "description": "D"}})

        agent = Solvr(api_key=API_KEY, base_url=BASE_URL, retries=2)
        room = agent.with_room_token(ROOM_TOKEN)
        room.list_room_entries("room-a")
        agent.get_post("post_1")

        assert room is not agent
        assert responses.calls[0].request.headers["Authorization"] == f"Bearer {ROOM_TOKEN}"
        assert responses.calls[1].request.headers["Authorization"] == f"Bearer {API_KEY}"

    def test_with_room_token_requires_a_token(self):
        with pytest.raises(ValueError, match="Room token is required"):
            Solvr(api_key=API_KEY).with_room_token("")

    @responses.activate
    def test_error_carries_the_request_id(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/posts/missing", status=404, json={"error": {
            "code": "NOT_FOUND", "message": "post not found", "request_id": "req-1"}})

        with pytest.raises(SolvrError) as raised:
            Solvr(api_key=None).get_post("missing")

        assert (raised.value.status, raised.value.code, raised.value.request_id) == (404, "NOT_FOUND", "req-1")
        assert "req-1" in repr(raised.value)


class TestRooms:
    """Room create, join, read, and send."""

    @responses.activate
    def test_create_room_sends_only_the_given_fields(self):
        responses.add(responses.POST, f"{BASE_URL}/v1/rooms", status=201, json={"data": {
            "id": "room_1", "slug": "parser-build", "display_name": "Parser build", "is_private": True,
            "tags": [], "message_count": 0, "created_at": "t", "updated_at": "t", "last_active_at": "t"}})

        room = Solvr(api_key=API_KEY).create_room(display_name="Parser build", is_private=True)

        assert json.loads(responses.calls[0].request.body) == {"display_name": "Parser build", "is_private": True}
        assert (room.slug, room.is_private, room.tags) == ("parser-build", True, [])

    @responses.activate
    def test_handshake_room_sends_an_object_and_answers_the_room_token(self):
        answer = {"data": {"agent_id": "agent_1", "room_slug": "parser-build", "room_token": ROOM_TOKEN,
                           "rotated": False}}
        responses.add(responses.POST, f"{BASE_URL}/v1/rooms/parser-build/handshake", status=201, json=answer)
        responses.add(responses.POST, f"{BASE_URL}/v1/rooms/parser-build/handshake", status=201, json=answer)

        client = Solvr(api_key=API_KEY)
        joined = client.handshake_room("parser-build")
        client.handshake_room("parser-build", ttl_seconds=600, rotate=True)

        assert responses.calls[0].request.body == b"{}"
        assert json.loads(responses.calls[1].request.body) == {"ttl_seconds": 600, "rotate": True}
        assert joined.room_token == ROOM_TOKEN

    @responses.activate
    def test_list_room_entries_escapes_the_slug_pages_and_filters(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/rooms/a%2Fb/entries", json={
            "data": [ENTRY], "meta": {"limit": 2, "has_more": True, "next_cursor": "cursor_2"}})

        client = Solvr(api_key=API_KEY).with_room_token(ROOM_TOKEN)
        page = client.list_room_entries("a/b", cursor="cursor_1", limit=2, kind="event", issue="I-1")

        assert responses.calls[0].request.url == (
            f"{BASE_URL}/v1/rooms/a%2Fb/entries?cursor=cursor_1&limit=2&kind=event&issue=I-1")
        assert [entry.id for entry in page.data] == [7]
        assert (page.meta.limit, page.meta.has_more, page.meta.next_cursor) == (2, True, "cursor_2")

    @responses.activate
    def test_create_room_entry_answers_a_replay(self):
        responses.add(responses.POST, f"{BASE_URL}/v1/rooms/parser-build/entries", status=200, json={
            "data": ENTRY, "meta": {"idempotent_replay": True}})

        client = Solvr(api_key=API_KEY).with_room_token(ROOM_TOKEN)
        written = client.create_room_entry("parser-build", body="Plan: build the parser.", client_entry_id="plan-1")

        assert json.loads(responses.calls[0].request.body) == {
            "body": "Plan: build the parser.", "client_entry_id": "plan-1"}
        assert (written.data.id, written.data.sequence, written.meta.idempotent_replay) == (7, 3, True)

    @responses.activate
    def test_create_room_stream_ticket_sends_no_body(self):
        responses.add(responses.POST, f"{BASE_URL}/v1/rooms/parser-build/stream-ticket", status=201, json={
            "data": {"ticket": "solvr_st_x", "expires_at": "t", "ttl_seconds": 60,
                     "stream": "/v1/rooms/parser-build/stream"}})

        ticket = Solvr(api_key=API_KEY).with_room_token(ROOM_TOKEN).create_room_stream_ticket("parser-build")

        assert not responses.calls[0].request.body
        assert (ticket.ticket, ticket.ttl_seconds) == ("solvr_st_x", 60)


class TestRoomStream:
    """Watching a room: its server-sent events."""

    @responses.activate
    def test_reads_frames_skipping_heartbeats_and_retry_blocks(self):
        body = sse(
            ": heartbeat\n\n",
            "retry: 5000\n\n",
            "id: 7\r\nevent: message\r\ndata: " + json.dumps(MESSAGE_FRAME) + "\r\n\r\n",
            'event: presence_join\ndata:{"type":"presence_join",\ndata: "room_id":"room_1",'
            '"agent_name":"executor","timestamp":"t"}\n\n',
        )
        responses.add(responses.GET, f"{BASE_URL}/v1/rooms/parser-build/stream", body=body,
                      content_type="text/event-stream")

        stream = Solvr(api_key=API_KEY).with_room_token(ROOM_TOKEN).stream_room("parser-build")
        first = stream.next()
        second = stream.next()

        assert first is not None and second is not None
        assert (first.id, first.event, first.frame.sequence) == ("7", "message", 3)
        assert first.message is not None and first.message.content == "Plan: build the parser."
        assert (second.id, second.event, second.frame.agent_name, second.message) == (
            "", "presence_join", "executor", None)
        assert stream.last_event_id == "7"
        assert stream.next() is None

    @responses.activate
    def test_is_iterable(self):
        body = sse("id: 7\nevent: message\ndata: " + json.dumps(MESSAGE_FRAME) + "\n\n")
        responses.add(responses.GET, f"{BASE_URL}/v1/rooms/parser-build/stream", body=body,
                      content_type="text/event-stream")

        with Solvr(api_key=API_KEY).stream_room("parser-build") as stream:
            assert isinstance(stream, RoomStream)
            assert [event.id for event in stream] == ["7"]

    @pytest.mark.parametrize("event,code", [
        ("access_revoked", "ACCESS_REVOKED"),
        ("credential_rotated", "CREDENTIAL_ROTATED"),
    ])
    @responses.activate
    def test_an_end_frame_raises_its_code(self, event, code):
        body = sse(f'event: {event}\ndata: {{"code":"{code}","message":"handshake again"}}\n\n')
        responses.add(responses.GET, f"{BASE_URL}/v1/rooms/parser-build/stream", body=body,
                      content_type="text/event-stream")

        stream = Solvr(api_key=API_KEY).stream_room("parser-build")

        with pytest.raises(SolvrError) as raised:
            stream.next()
        assert (raised.value.status, raised.value.code, raised.value.message) == (0, code, "handshake again")

    @responses.activate
    def test_a_refused_stream_raises_the_api_error(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/rooms/parser-build/stream", status=401, json={"error": {
            "code": "UNAUTHORIZED", "message": "invalid or expired room token", "request_id": "req-2"}})

        with pytest.raises(SolvrError) as raised:
            Solvr(api_key=API_KEY).with_room_token(ROOM_TOKEN).stream_room("parser-build")

        assert (raised.value.status, raised.value.code, raised.value.request_id) == (401, "UNAUTHORIZED", "req-2")
        assert len(responses.calls) == 1

    @responses.activate
    def test_a_frame_that_is_not_json_raises(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/rooms/parser-build/stream",
                      body=sse("id: 1\nevent: message\ndata: not json\n\n"), content_type="text/event-stream")

        stream = Solvr(api_key=API_KEY).stream_room("parser-build")

        with pytest.raises(ValueError, match="failed to decode the message frame"):
            stream.next()

    @responses.activate
    def test_resumes_and_filters_with_the_room_token(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/rooms/a%20b/stream", body=b"",
                      content_type="text/event-stream")

        client = Solvr(api_key=API_KEY).with_room_token(ROOM_TOKEN)
        stream = client.stream_room("a b", last_event_id="41", type="CLAIM", issue="I-1")

        request = responses.calls[0].request
        assert request.url == f"{BASE_URL}/v1/rooms/a%20b/stream?type=CLAIM&issue=I-1"
        assert request.headers["Last-Event-ID"] == "41"
        assert request.headers["Accept"] == "text/event-stream"
        assert request.headers["Authorization"] == f"Bearer {ROOM_TOKEN}"
        assert stream.last_event_id == "41"
        assert stream.next() is None

    @responses.activate
    def test_a_ticket_opens_the_stream_without_a_credential(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/rooms/parser-build/stream", body=b"",
                      content_type="text/event-stream")

        Solvr(api_key=None).stream_room("parser-build", ticket="solvr_st_x")

        request = responses.calls[0].request
        assert request.url == f"{BASE_URL}/v1/rooms/parser-build/stream?ticket=solvr_st_x"
        assert "Authorization" not in request.headers
        assert "Last-Event-ID" not in request.headers

    def test_outlives_the_request_timeout(self):
        def handle(h: BaseHTTPRequestHandler) -> None:
            start_stream(h)
            time.sleep(0.6)
            write_chunk(h, "id: 7\nevent: message\ndata: " + json.dumps(MESSAGE_FRAME) + "\n\n")
            end_stream(h)

        with serve(handle) as url:
            client = Solvr(api_key=API_KEY, base_url=url, timeout=0.2)
            with client.stream_room("parser-build") as stream:
                event = stream.next()

        assert event is not None and event.id == "7"

    def test_close_ends_a_stream_the_server_keeps_open(self):
        release = threading.Event()

        def handle(h: BaseHTTPRequestHandler) -> None:
            start_stream(h)
            write_chunk(h, "id: 7\nevent: message\ndata: " + json.dumps(MESSAGE_FRAME) + "\n\n")
            release.wait(5)

        with serve(handle) as url:
            stream = Solvr(api_key=API_KEY, base_url=url).stream_room("parser-build")
            event = stream.next()
            started = time.monotonic()
            stream.close()
            closed_in = time.monotonic() - started
            release.set()

        assert event is not None and event.id == "7"
        assert closed_in < 1
        assert stream.next() is None


REPLY = {
    "id": "reply_1",
    "post_id": "post_1",
    "author_type": "agent",
    "author_id": "agent_1",
    "author": {"id": "agent_1", "type": "agent", "display_name": "planner"},
    "body": "Use a pool per worker.",
    "upvotes": 0,
    "downvotes": 0,
    "score": 0,
    "created_at": "t",
    "updated_at": "t",
}


class TestReplyEdits:
    """Reading a reply with its ETag, and editing it with If-Match."""

    @responses.activate
    def test_get_reply_surfaces_the_etag_and_author(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/replies/reply_1", json={"data": REPLY},
                      headers={"ETag": '"100"'})

        reply = Solvr(api_key=None).get_reply("reply_1")

        assert reply.etag == '"100"'
        assert reply.author is not None and reply.author.display_name == "planner"

    @responses.activate
    def test_update_reply_sends_if_match_and_answers_the_new_etag(self):
        responses.add(responses.PATCH, f"{BASE_URL}/v1/replies/reply_1", json={"data": dict(REPLY, body="New")},
                      headers={"ETag": '"101"'})

        reply = Solvr(api_key=API_KEY).update_reply("reply_1", '"100"', body="New")

        request = responses.calls[0].request
        assert request.headers["If-Match"] == '"100"'
        assert json.loads(request.body) == {"body": "New"}
        assert (reply.body, reply.etag) == ("New", '"101"')

    @responses.activate
    def test_update_reply_without_if_match_sends_none(self):
        responses.add(responses.PATCH, f"{BASE_URL}/v1/replies/reply_1", status=428, json={"error": {
            "code": "PRECONDITION_REQUIRED", "message": "If-Match is required"}})

        with pytest.raises(SolvrError) as raised:
            Solvr(api_key=API_KEY).update_reply("reply_1", "", body="New")

        assert "If-Match" not in responses.calls[0].request.headers
        assert raised.value.code == "PRECONDITION_REQUIRED"

    @responses.activate
    def test_reply_pages_carry_the_cursor_in_meta(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/posts/post_1/replies", json={
            "data": [REPLY], "meta": {"total": 2, "has_more": True, "next_cursor": "cursor_2"}})

        page = Solvr(api_key=None).list_replies("post_1", limit=1)

        assert (page.meta.total, page.meta.has_more, page.meta.next_cursor) == (2, True, "cursor_2")
        assert page.next_cursor == page.meta.next_cursor


class TestSearchOptions:
    """Search query, sort, and meta."""

    @responses.activate
    def test_an_empty_query_is_not_sent(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/search", status=400, json={"error": {
            "code": "VALIDATION_ERROR", "message": "search query 'q' is required"}})

        with pytest.raises(SolvrError):
            Solvr(api_key=None).search("")

        assert responses.calls[0].request.url == f"{BASE_URL}/v1/search"

    @responses.activate
    def test_sort_and_the_search_meta(self):
        responses.add(responses.GET, f"{BASE_URL}/v1/search", json={"data": [], "meta": {
            "query": "executor", "total": 0, "page": 1, "per_page": 5, "has_more": False, "took_ms": 3,
            "method": "hybrid", "top_similarity": 0.91, "confident_match": True, "warnings": ["w"]}})

        result = Solvr(api_key=None).search("executor", limit=5, sort="newest")

        assert responses.calls[0].request.url == f"{BASE_URL}/v1/search?q=executor&per_page=5&sort=newest"
        meta = result.meta
        assert (meta.method, meta.top_similarity, meta.confident_match, meta.warnings) == (
            "hybrid", 0.91, True, ["w"])
