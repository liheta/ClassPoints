const API_BASE = "/api";

export async function apiGet(path) {
  return request(path);
}

export async function apiPost(path, body = {}) {
  return request(path, { method: "POST", body: JSON.stringify(body) });
}

export async function apiPut(path, body = {}) {
  return request(path, { method: "PUT", body: JSON.stringify(body) });
}

export async function apiDelete(path) {
  return request(path, { method: "DELETE" });
}

async function request(path, options = {}) {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
    ...options,
  });

  const text = await response.text();
  const payload = text ? JSON.parse(text) : null;
  if (!response.ok) {
    throw new Error(payload?.error || "请求失败");
  }
  return payload;
}
