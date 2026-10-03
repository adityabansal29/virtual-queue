#!/usr/bin/env python3
"""Run a small real-client queue simulation without third-party packages."""
import argparse
import json
import time
import urllib.parse
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from http.cookiejar import CookieJar


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def http_error_302(self, req, fp, code, msg, headers):
        return fp

    http_error_301 = http_error_303 = http_error_307 = http_error_302


def client():
    return urllib.request.build_opener(NoRedirect(), urllib.request.HTTPCookieProcessor(CookieJar()))


def request(opener, url, timeout):
    with opener.open(url, timeout=timeout) as response:
        return response.status, response.headers, response.read()


def user(api, event_id, number, timeout):
    opener = client()
    join = f"{api}/queue/join?eventId={urllib.parse.quote(event_id)}"
    status, headers, _ = request(opener, join, timeout)
    location = headers.get("Location", "")
    ticket = urllib.parse.parse_qs(urllib.parse.urlparse(location).query).get("ticket", [""])[0]
    if status != 302 or not ticket:
        raise RuntimeError(f"user {number}: join returned {status} without ticket")

    mode = "poll"
    for attempt in range(1, 121):
        status_url = f"{api}/queue/status/{urllib.parse.quote(ticket)}?mode={mode}"
        if mode == "sse":
            status, _, body = request(opener, status_url, timeout)
            text = body.decode()
            if '"type":"admitted"' in text:
                return number, ticket, "sse", attempt
        else:
            status, _, body = request(opener, status_url, timeout)
            data = json.loads(body)
            if data.get("type") == "admitted":
                return number, ticket, "poll", attempt
            if data.get("upgrade_to_sse"):
                mode = "sse"
        if status != 200:
            raise RuntimeError(f"user {number}: {mode} returned {status}")
        time.sleep(1 if mode == "sse" else 5)
    raise TimeoutError(f"user {number}: not admitted after 120 checks")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--api", default="http://localhost:8080")
    parser.add_argument("--event", default=f"sim-{int(time.time())}")
    parser.add_argument("--count", type=int, default=20)
    parser.add_argument("--timeout", type=float, default=10)
    args = parser.parse_args()
    started = time.time()
    print(f"joining {args.count} users to {args.event} via {args.api}")
    with ThreadPoolExecutor(max_workers=args.count) as pool:
        jobs = [pool.submit(user, args.api, args.event, i, args.timeout) for i in range(1, args.count + 1)]
        for job in as_completed(jobs):
            try:
                number, ticket, mode, checks = job.result()
                print(f"user={number:02d} admitted transport={mode} checks={checks} ticket={ticket}")
            except Exception as exc:
                print(f"ERROR {exc}")
                raise
    print(f"completed {args.count} users in {time.time() - started:.1f}s")


if __name__ == "__main__":
    main()
