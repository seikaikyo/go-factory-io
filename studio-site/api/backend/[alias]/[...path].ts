import type { IncomingMessage, ServerResponse } from "node:http";

/* 後端只收代理流量。瀏覽器打同源 /api/backend/<alias>/<path>，
   這裡帶 X-Origin-Key 轉給 Render。alias 對照表在 BACKEND_ORIGINS
   （alias=origin 逗號分隔），金鑰在 ORIGIN_KEY，兩個都是伺服器端變數。
   回應用串流轉送，chat 的 SSE 才不會被整包緩衝。 */
const ORIGINS: Record<string, string> = Object.fromEntries(
  (process.env.BACKEND_ORIGINS ?? "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
    .map((s) => {
      const i = s.indexOf("=");
      return [s.slice(0, i), s.slice(i + 1).replace(/\/$/, "")];
    }),
);

const PASS_REQUEST = ["content-type", "accept", "accept-language", "authorization", "cookie", "x-api-key", "x-passcode", "x-requested-with"];
const PASS_RESPONSE = ["content-type", "content-disposition", "etag", "last-modified"];

function readBody(req: IncomingMessage): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on("data", (c: Buffer) => chunks.push(c));
    req.on("end", () => resolve(Buffer.concat(chunks)));
    req.on("error", reject);
  });
}

export default async function handler(req: IncomingMessage & { query?: Record<string, string | string[]> }, res: ServerResponse) {
  const q = req.query ?? {};
  const alias = String(q.alias ?? "");
  const rawPath = Array.isArray(q.path) ? q.path.join("/") : String(q.path ?? "");
  const parts = rawPath.split("/").filter(Boolean).map(encodeURIComponent);
  const origin = ORIGINS[alias];
  if (!origin) {
    res.statusCode = 404;
    res.setHeader("content-type", "application/json");
    res.end(JSON.stringify({ detail: "unknown backend" }));
    return;
  }
  const incoming = new URL(req.url ?? "/", "http://local");
  const url = new URL(`${origin}/${parts.join("/")}`);
  url.search = incoming.search;

  const headers = new Headers();
  for (const h of PASS_REQUEST) {
    const v = req.headers[h];
    if (typeof v === "string") headers.set(h, v);
  }
  const key = process.env.ORIGIN_KEY;
  if (key) headers.set("x-origin-key", key);
  const xff = req.headers["x-forwarded-for"];
  if (typeof xff === "string") headers.set("x-forwarded-for", xff);

  const method = req.method ?? "GET";
  const abort = new AbortController();
  const init: RequestInit = { method, headers, redirect: "manual", signal: abort.signal };
  if (method !== "GET" && method !== "HEAD") init.body = await readBody(req);

  const upstream = await fetch(url, init);
  res.statusCode = upstream.status;
  res.setHeader("cache-control", "no-store");
  for (const h of PASS_RESPONSE) {
    const v = upstream.headers.get(h);
    if (v) res.setHeader(h, v);
  }
  const cookies = upstream.headers.getSetCookie();
  if (cookies.length) res.setHeader("set-cookie", cookies);
  if (!upstream.body) {
    res.end();
    return;
  }
  res.on("close", () => {
    if (!res.writableFinished) abort.abort();
  });
  const reader = upstream.body.getReader();
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      res.write(value);
    }
  } catch (e) {
    if (!abort.signal.aborted) throw e;
  }
  res.end();
}
