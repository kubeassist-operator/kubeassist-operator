#!/usr/bin/env python3
"""
KubeAssist Agent — minimal reference implementation.

Reads AGENT_API_KEY (or the env var named by AGENT_API_KEY_ENV_VAR)
and exposes two endpoints:

  GET  /health   → liveness / readiness probe
  POST /invoke   → send a prompt to any OpenAI-compatible LLM API

Environment variables:
  AGENT_API_KEY          LLM API key (injected by kubeassist-operator)
  AGENT_API_BASE         OpenAI-compatible base URL
                         default: https://api.openai.com/v1
  AGENT_MODEL            Model name  default: gpt-4o-mini
  AGENT_MODE             Arbitrary mode string (informational)
  AGENT_MAX_JOBS         Max concurrent requests  default: 100
"""

import os
import time
import logging
import asyncio
from contextlib import asynccontextmanager

import httpx
from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel

# ── Logging ───────────────────────────────────────────────────────────────────
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s  %(levelname)-8s  %(message)s",
)
log = logging.getLogger("kubeassist-agent")

# ── Config from environment ───────────────────────────────────────────────────
API_KEY   = os.environ.get("AGENT_API_KEY", "")
API_BASE  = os.environ.get("AGENT_API_BASE", "https://api.openai.com/v1").rstrip("/")
MODEL     = os.environ.get("AGENT_MODEL", "gpt-4o-mini")
MODE      = os.environ.get("AGENT_MODE", "default")
MAX_JOBS  = int(os.environ.get("AGENT_MAX_JOBS", "100"))

START_TIME = time.time()

# Semaphore to cap concurrent LLM calls
_sem: asyncio.Semaphore


@asynccontextmanager
async def lifespan(app: FastAPI):
    global _sem
    _sem = asyncio.Semaphore(MAX_JOBS)
    log.info(f"KubeAssist agent starting — model={MODEL} base={API_BASE} mode={MODE}")
    if not API_KEY:
        log.warning("AGENT_API_KEY is not set — /invoke will return 503")
    yield
    log.info("KubeAssist agent shutting down")


app = FastAPI(
    title="KubeAssist Agent",
    description="Minimal reference agent for the kubeassist-operator",
    version="0.1.0",
    lifespan=lifespan,
)

# ── Models ────────────────────────────────────────────────────────────────────

class InvokeRequest(BaseModel):
    prompt: str
    system: str = "You are a helpful Kubernetes assistant."
    model: str = ""          # override per-request; falls back to AGENT_MODEL
    max_tokens: int = 1024
    temperature: float = 0.2


class InvokeResponse(BaseModel):
    answer: str
    model: str
    usage: dict


# ── Endpoints ─────────────────────────────────────────────────────────────────

@app.get("/health")
async def health():
    """Liveness and readiness probe."""
    return {
        "status": "ok",
        "uptime_seconds": round(time.time() - START_TIME, 1),
        "model": MODEL,
        "mode": MODE,
        "api_key_set": bool(API_KEY),
    }


@app.post("/invoke", response_model=InvokeResponse)
async def invoke(req: InvokeRequest):
    """Send a prompt to the configured LLM and return the response."""
    if not API_KEY:
        raise HTTPException(
            status_code=503,
            detail="AGENT_API_KEY is not configured. "
                   "Create a Secret and reference it in spec.apiKeySecretRef.",
        )

    model = req.model or MODEL
    payload = {
        "model": model,
        "messages": [
            {"role": "system",  "content": req.system},
            {"role": "user",    "content": req.prompt},
        ],
        "max_tokens":  req.max_tokens,
        "temperature": req.temperature,
    }

    async with _sem:
        try:
            async with httpx.AsyncClient(timeout=120.0) as client:
                resp = await client.post(
                    f"{API_BASE}/chat/completions",
                    headers={
                        "Authorization": f"Bearer {API_KEY}",
                        "Content-Type":  "application/json",
                    },
                    json=payload,
                )
        except httpx.TimeoutException:
            raise HTTPException(status_code=504, detail="LLM API request timed out")
        except httpx.RequestError as e:
            raise HTTPException(status_code=502, detail=f"LLM API connection error: {e}")

    if resp.status_code != 200:
        log.error(f"LLM API error {resp.status_code}: {resp.text[:300]}")
        raise HTTPException(
            status_code=resp.status_code,
            detail=f"LLM API returned {resp.status_code}: {resp.text[:300]}",
        )

    data = resp.json()
    answer = data["choices"][0]["message"]["content"]
    usage  = data.get("usage", {})

    log.info(f"invoke model={model} prompt_tokens={usage.get('prompt_tokens','?')} "
             f"completion_tokens={usage.get('completion_tokens','?')}")

    return InvokeResponse(answer=answer, model=model, usage=usage)


@app.get("/")
async def root():
    return JSONResponse({
        "name":    "kubeassist-agent",
        "version": "0.1.0",
        "docs":    "/docs",
        "health":  "/health",
        "invoke":  "/invoke",
    })
