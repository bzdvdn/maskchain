#!/usr/bin/env python3
"""Minimal MaskChain caller using the OpenAI Python SDK.

Install:  pip install openai
Run:      python example.py
Env:      MASKCHAIN_URL (default http://localhost:8080/api/v1)
          MASKCHAIN_KEY (default sk-test-default)
          MASKCHAIN_MODEL (default llama3.2)
"""

import os

from openai import OpenAI

base_url = os.getenv("MASKCHAIN_URL", "http://localhost:8080/api/v1")
api_key = os.getenv("MASKCHAIN_KEY", "sk-test-default")
model = os.getenv("MASKCHAIN_MODEL", "llama3.2")

client = OpenAI(base_url=base_url, api_key=api_key)

response = client.chat.completions.create(
    model=model,
    messages=[
        {
            "role": "user",
            "content": "Summarize this note: contact Alice at alice@example.com or +1-555-0100.",
        }
    ],
)

# MaskChain masks before the provider call and unmasks on the way back,
# so the client sees the original values.
print(response.choices[0].message.content)
