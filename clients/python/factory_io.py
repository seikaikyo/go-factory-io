"""
go-factory-io Python Client

Async HTTP client for the go-factory-io REST API.
Designed for integration with smart-factory-demo's FastAPI backend.

Usage:
    from factory_io import FactoryIOClient

    async with FactoryIOClient("http://localhost:8080") as client:
        status = await client.get_status()
        svs = await client.list_sv()
        temp = await client.get_sv(1002)
"""

from __future__ import annotations

import asyncio
import json
from contextlib import asynccontextmanager
from dataclasses import dataclass
from typing import Any, AsyncIterator, Optional

import httpx


@dataclass
class EquipmentStatus:
    comm_state: str
    control_state: str
    communicating: bool
    online: bool
    transport: str


@dataclass
class StatusVariable:
    svid: int
    name: str
    value: Any
    units: str


@dataclass
class EquipmentConstant:
    ecid: int
    name: str
    value: Any
    units: str


@dataclass
class AlarmInfo:
    alid: int
    name: str
    text: str
    state: str
    enabled: bool


@dataclass
class CommandResult:
    command: str
    status: str
    code: int


class FactoryIOError(Exception):
    """Raised when the go-factory-io API returns an error."""
    def __init__(self, status_code: int, message: str):
        self.status_code = status_code
        self.message = message
        super().__init__(f"[{status_code}] {message}")


class FactoryIOClient:
    """Async HTTP client for go-factory-io REST API."""

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        token: Optional[str] = None,
        timeout: float = 10.0,
    ):
        headers = {}
        if token:
            headers["Authorization"] = f"Bearer {token}"
        self._client = httpx.AsyncClient(
            base_url=base_url,
            headers=headers,
            timeout=timeout,
        )

    async def __aenter__(self):
        return self

    async def __aexit__(self, *args):
        await self.close()

    async def close(self):
        await self._client.aclose()

    async def _get(self, path: str) -> dict:
        resp = await self._client.get(path)
        data = resp.json()
        if not data.get("success"):
            err = data.get("error", {})
            raise FactoryIOError(err.get("code", resp.status_code), err.get("message", "unknown"))
        return data["data"]

    async def _put(self, path: str, body: dict) -> dict:
        resp = await self._client.put(path, json=body)
        data = resp.json()
        if not data.get("success"):
            err = data.get("error", {})
            raise FactoryIOError(err.get("code", resp.status_code), err.get("message", "unknown"))
        return data["data"]

    async def _post(self, path: str, body: dict) -> dict:
        resp = await self._client.post(path, json=body)
        data = resp.json()
        if not data.get("success"):
            err = data.get("error", {})
            raise FactoryIOError(err.get("code", resp.status_code), err.get("message", "unknown"))
        return data["data"]

    # --- Health ---

    async def health(self) -> dict:
        return await self._get("/health")

    # --- Equipment Status ---

    async def get_status(self) -> EquipmentStatus:
        d = await self._get("/api/status")
        return EquipmentStatus(
            comm_state=d["commState"],
            control_state=d["controlState"],
            communicating=d["communicating"],
            online=d["online"],
            transport=d["transport"],
        )

    # --- Status Variables ---

    async def list_sv(self) -> list[StatusVariable]:
        items = await self._get("/api/sv")
        return [StatusVariable(svid=s["svid"], name=s["name"], value=s["value"], units=s["units"]) for s in items]

    async def get_sv(self, svid: int) -> StatusVariable:
        d = await self._get(f"/api/sv/{svid}")
        return StatusVariable(svid=d["svid"], name=d["name"], value=d["value"], units=d["units"])

    # --- Equipment Constants ---

    async def list_ec(self) -> list[EquipmentConstant]:
        items = await self._get("/api/ec")
        return [EquipmentConstant(ecid=e["ecid"], name=e["name"], value=e["value"], units=e["units"]) for e in items]

    async def get_ec(self, ecid: int) -> EquipmentConstant:
        d = await self._get(f"/api/ec/{ecid}")
        return EquipmentConstant(ecid=d["ecid"], name=d["name"], value=d["value"], units=d["units"])

    async def set_ec(self, ecid: int, value: Any) -> dict:
        return await self._put(f"/api/ec/{ecid}", {"value": value})

    # --- Alarms ---

    async def list_alarms(self) -> list[AlarmInfo]:
        items = await self._get("/api/alarms")
        return [AlarmInfo(
            alid=a["alid"], name=a["name"], text=a["text"],
            state=a["state"], enabled=a["enabled"],
        ) for a in items]

    async def list_active_alarms(self) -> list[AlarmInfo]:
        items = await self._get("/api/alarms/active")
        return [AlarmInfo(alid=a["alid"], name=a["name"], text=a.get("text", ""), state="SET", enabled=True) for a in items]

    # --- Remote Commands ---

    async def send_command(self, command: str, params: Optional[dict] = None) -> CommandResult:
        d = await self._post("/api/command", {"command": command, "params": params or {}})
        return CommandResult(command=d["command"], status=d["status"], code=d["code"])

    # --- SSE Event Stream ---

    async def events(self) -> AsyncIterator[dict]:
        """Subscribe to real-time equipment events via SSE.

        Usage:
            async for event in client.events():
                print(event["type"], event["data"])
        """
        async with self._client.stream("GET", "/api/events") as resp:
            async for line in resp.aiter_lines():
                if line.startswith("data: "):
                    try:
                        yield json.loads(line[6:])
                    except json.JSONDecodeError:
                        continue


# --- Sync wrapper for non-async code ---

class FactoryIOSyncClient:
    """Synchronous wrapper for environments without asyncio (e.g., Jupyter, scripts)."""

    def __init__(self, base_url: str = "http://localhost:8080", token: Optional[str] = None):
        headers = {}
        if token:
            headers["Authorization"] = f"Bearer {token}"
        self._client = httpx.Client(base_url=base_url, headers=headers, timeout=10.0)

    def close(self):
        self._client.close()

    def _get(self, path: str) -> dict:
        resp = self._client.get(path)
        data = resp.json()
        if not data.get("success"):
            err = data.get("error", {})
            raise FactoryIOError(err.get("code", resp.status_code), err.get("message", "unknown"))
        return data["data"]

    def _put(self, path: str, body: dict) -> dict:
        resp = self._client.put(path, json=body)
        data = resp.json()
        if not data.get("success"):
            err = data.get("error", {})
            raise FactoryIOError(err.get("code", resp.status_code), err.get("message", "unknown"))
        return data["data"]

    def _post(self, path: str, body: dict) -> dict:
        resp = self._client.post(path, json=body)
        data = resp.json()
        if not data.get("success"):
            err = data.get("error", {})
            raise FactoryIOError(err.get("code", resp.status_code), err.get("message", "unknown"))
        return data["data"]

    def health(self) -> dict:
        return self._get("/health")

    def get_status(self) -> EquipmentStatus:
        d = self._get("/api/status")
        return EquipmentStatus(
            comm_state=d["commState"], control_state=d["controlState"],
            communicating=d["communicating"], online=d["online"], transport=d["transport"],
        )

    def list_sv(self) -> list[StatusVariable]:
        items = self._get("/api/sv")
        return [StatusVariable(svid=s["svid"], name=s["name"], value=s["value"], units=s["units"]) for s in items]

    def get_sv(self, svid: int) -> StatusVariable:
        d = self._get(f"/api/sv/{svid}")
        return StatusVariable(svid=d["svid"], name=d["name"], value=d["value"], units=d["units"])

    def list_ec(self) -> list[EquipmentConstant]:
        items = self._get("/api/ec")
        return [EquipmentConstant(ecid=e["ecid"], name=e["name"], value=e["value"], units=e["units"]) for e in items]

    def get_ec(self, ecid: int) -> EquipmentConstant:
        d = self._get(f"/api/ec/{ecid}")
        return EquipmentConstant(ecid=d["ecid"], name=d["name"], value=d["value"], units=d["units"])

    def set_ec(self, ecid: int, value: Any) -> dict:
        return self._put(f"/api/ec/{ecid}", {"value": value})

    def list_alarms(self) -> list[AlarmInfo]:
        items = self._get("/api/alarms")
        return [AlarmInfo(alid=a["alid"], name=a["name"], text=a["text"], state=a["state"], enabled=a["enabled"]) for a in items]

    def send_command(self, command: str, params: Optional[dict] = None) -> CommandResult:
        d = self._post("/api/command", {"command": command, "params": params or {}})
        return CommandResult(command=d["command"], status=d["status"], code=d["code"])
