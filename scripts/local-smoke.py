#!/usr/bin/env python3
"""Headless local-stack smoke test using the real Go OIDC callback path."""

from __future__ import annotations

import http.cookiejar
import json
import re
import sys
from urllib.error import HTTPError
from urllib.parse import parse_qs, urlencode, urljoin, urlparse
from urllib.request import (
    HTTPRedirectHandler,
    HTTPCookieProcessor,
    Request,
    build_opener,
)


BASE_URL = "http://web.seshatops.localhost:5173"
TENANT_ID = "11111111-1111-4111-8111-111111111111"
NEGATIVE_TENANT_ID = "22222222-2222-4222-8222-222222222222"


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def request(opener, url, data=None):
    req = Request(url, data=data, headers={"Accept": "application/json, text/html"})
    try:
        with opener.open(req) as response:
            return response.status, dict(response.headers), response.read()
    except HTTPError as error:
        return error.code, dict(error.headers), error.read()


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def header(headers, name):
    return next((value for key, value in headers.items() if key.lower() == name.lower()), None)


def main():
    jar = http.cookiejar.CookieJar()
    opener = build_opener(HTTPCookieProcessor(jar), NoRedirect())

    status, _, body = request(opener, BASE_URL + "/")
    require(status == 200 and b"Northstar" in body, "web entrypoint is not serving the console")

    status, _, body = request(opener, BASE_URL + "/auth/session")
    require(status == 401 and json.loads(body)["error"] == "unauthenticated", "auth route is not Go-owned")
    status, _, body = request(opener, BASE_URL + f"/v1/tenants/{TENANT_ID}/inventory")
    require(status == 401 and json.loads(body)["error"] == "unauthenticated", "API route is not Go-owned")
    status, _, body = request(opener, BASE_URL + "/metrics")
    require(status == 401 and b"seshatops_" not in body, "metrics leaked without a session")

    status, headers, _ = request(opener, BASE_URL + "/auth/login")
    authorize_url = header(headers, "Location")
    require(status == 302 and authorize_url is not None, "login did not redirect to OIDC")
    query = parse_qs(urlparse(authorize_url).query)
    require(query.get("code_challenge_method") == ["S256"], "login did not use PKCE")
    require(query.get("redirect_uri") == [BASE_URL + "/auth/callback"], "callback route is not same-origin")

    status, _, body = request(opener, authorize_url)
    require(status == 200, "OIDC authorization page is not reachable")
    form = re.search(rb"<form\b[^>]*>", body, re.IGNORECASE)
    require(form is not None, "OIDC authorization page has no login form")
    action = re.search(rb'\saction=["\']([^"\']+)', form.group(0), re.IGNORECASE)
    login_url = urljoin(authorize_url, action.group(1).decode() if action else authorize_url)
    status, headers, _ = request(
        opener,
        login_url,
        data=urlencode({"username": "northstar-demo-operator"}).encode(),
    )
    callback_url = header(headers, "Location")
    require(status == 302 and callback_url is not None, "OIDC login did not return an authorization code")
    require(urlparse(callback_url).netloc == urlparse(BASE_URL).netloc, "OIDC callback left the web origin")

    status, headers, _ = request(opener, callback_url)
    require(status == 302 and header(headers, "Location") is not None, "Go OIDC callback did not establish a session")
    status, _, body = request(opener, BASE_URL + "/auth/session")
    session = json.loads(body)
    require(status == 200 and session["principal_id"] == "northstar-demo-operator", "session identity is wrong")

    status, _, body = request(opener, BASE_URL + f"/v1/tenants/{TENANT_ID}/inventory")
    snapshot = json.loads(body)
    require(status == 200 and snapshot["tenant_id"] == TENANT_ID, "authenticated web-to-Go read failed")
    status, _, _ = request(opener, BASE_URL + f"/v1/tenants/{NEGATIVE_TENANT_ID}/inventory")
    require(status == 403, "cross-tenant read was not denied")

    status, headers, body = request(opener, BASE_URL + "/metrics")
    require(status == 200, "release observer metrics read failed")
    require(header(headers, "Content-Type").startswith("text/plain; version=0.0.4"), "metrics content type is wrong")
    require(header(headers, "X-Correlation-ID") is not None, "metrics response has no correlation ID")
    require(b"seshatops_runtime_ready 1" in body and b"seshatops_outbox_backlog_records_pending" in body, "metrics surface is incomplete")
    require(TENANT_ID.encode() not in body and NEGATIVE_TENANT_ID.encode() not in body, "metrics leaked tenant identity")

    print("local stack smoke passed: web routing, PKCE callback, session, tenant isolation, metrics access")


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, ValueError, KeyError) as error:
        print(f"local stack smoke failed: {error}", file=sys.stderr)
        raise SystemExit(1)
