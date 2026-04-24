# pipeline/tests/test_aws_etag_304.py
from __future__ import annotations

import http.server
import threading
from pathlib import Path

import pytest

from ingest import aws_common


class _HandlerFactory:
    def __init__(self, etag: str):
        self.etag = etag
        self.hit_count = 0

    def __call__(self, *args, **kwargs):
        factory = self
        class _H(http.server.BaseHTTPRequestHandler):
            def do_HEAD(self):
                factory.hit_count += 1
                inm = self.headers.get("If-None-Match")
                if inm == factory.etag:
                    self.send_response(304)
                    self.end_headers()
                    return
                self.send_response(200)
                self.send_header("ETag", factory.etag)
                self.send_header("Content-Length", "0")
                self.end_headers()

            def log_message(self, *a, **kw):
                pass

            do_GET = do_HEAD
        return _H(*args, **kwargs)


def test_etag_skips_on_304(tmp_path: Path) -> None:
    etag = '"abc-123"'
    factory = _HandlerFactory(etag)
    srv = http.server.HTTPServer(("127.0.0.1", 0), factory)
    t = threading.Thread(target=srv.serve_forever, daemon=True)
    t.start()
    try:
        url = f"http://127.0.0.1:{srv.server_port}/offer.json"
        cache_path = tmp_path / "etags.json"

        # Seed the cache with the ETag the server will echo.
        from ingest._etag_cache import EtagCache
        c = EtagCache(cache_path)
        c.set(url, etag)
        c.save()

        with pytest.raises(aws_common.NotModified):
            aws_common.fetch_with_etag(url, etag_cache=EtagCache(cache_path))
        assert factory.hit_count == 1
    finally:
        srv.shutdown()
