"""A local HTTP server for the tests that need a real socket (the contract tests and the room stream)."""

import threading
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Callable, Dict, Iterator, Optional

Handle = Callable[[BaseHTTPRequestHandler], None]


@contextmanager
def serve(handle: Handle) -> Iterator[str]:
    """Serves every request with handle; yields the server's base URL."""

    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def do_GET(self) -> None:
            handle(self)

        do_POST = do_PATCH = do_PUT = do_DELETE = do_GET

        def log_message(self, format: str, *args: object) -> None:
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = True
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_address[1]}"
    finally:
        server.shutdown()
        server.server_close()


def read_body(h: BaseHTTPRequestHandler) -> bytes:
    """The request body (the SDK always sends a Content-Length)."""
    length = int(h.headers.get("Content-Length") or 0)
    return h.rfile.read(length) if length else b""


def send_body(
    h: BaseHTTPRequestHandler,
    status: int,
    content_type: str,
    body: bytes,
    headers: Optional[Dict[str, str]] = None,
) -> None:
    """Answers with a whole body, then closes the connection."""
    h.send_response(status)
    h.send_header("Content-Type", content_type)
    h.send_header("Content-Length", str(len(body)))
    for name, value in (headers or {}).items():
        h.send_header(name, value)
    h.send_header("Connection", "close")
    h.end_headers()
    h.wfile.write(body)


def start_stream(h: BaseHTTPRequestHandler) -> None:
    """Starts a chunked event stream, as the API's Go server sends one."""
    h.send_response(200)
    h.send_header("Content-Type", "text/event-stream")
    h.send_header("Transfer-Encoding", "chunked")
    h.send_header("Connection", "close")
    h.end_headers()
    h.wfile.flush()


def write_chunk(h: BaseHTTPRequestHandler, text: str) -> None:
    data = text.encode("utf-8")
    h.wfile.write(f"{len(data):x}\r\n".encode("ascii") + data + b"\r\n")
    h.wfile.flush()


def end_stream(h: BaseHTTPRequestHandler) -> None:
    h.wfile.write(b"0\r\n\r\n")
    h.wfile.flush()
