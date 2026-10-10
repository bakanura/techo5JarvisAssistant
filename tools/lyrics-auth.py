#!/usr/bin/env python3
"""Make the login Home Assistant's jarvis_lyrics command sends to the music server (docs/lyrics.md).

OpenSubsonic servers such as Navidrome take a token, the MD5 of the password and a salt, in place of the
password itself. This reads the password on stdin, so it never sits on a command line, and writes one
line for Home Assistant's secrets.yaml:

    secret-tool lookup service navidrome user jarvis-lyrics | python3 tools/lyrics-auth.py jarvis-lyrics

The token still logs in as that user; keep the line where the password would go, not in a repo.
"""
import hashlib
import secrets
import sys
import urllib.parse


def main():
    if len(sys.argv) != 2:
        sys.exit('usage: lyrics-auth.py USER < password')
    user = sys.argv[1]
    password = sys.stdin.readline().rstrip('\r\n')
    if not password:
        sys.exit('no password on stdin')
    salt = secrets.token_hex(8)
    token = hashlib.md5((password + salt).encode()).hexdigest()
    auth = urllib.parse.urlencode({'u': user, 't': token, 's': salt, 'v': '1.16.1', 'c': 'jarvis-show', 'f': 'json'})
    print(f'jarvis_lyrics_auth: "{auth}"')


if __name__ == '__main__':
    main()
