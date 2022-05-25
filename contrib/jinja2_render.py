#!/usr/bin/env python3

import base64
import jinja2
import os
import sys

def to_bytes(v):
  return v if isinstance(v, bytes) else v.encode()

def b64encode(v):
  return base64.b64encode(to_bytes(v)).decode(encoding='utf-8')

def b64decode(v):
  return base64.b64decode(v).decode(encoding='utf-8')

env = jinja2.Environment()
env.filters['b64encode'] = b64encode
env.filters['b64decode'] = b64decode
template = env.from_string(sys.stdin.read())
print(template.render(**os.environ))
