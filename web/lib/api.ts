import { auth } from "@/auth";

const metadataIdentityURL =
  "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity";

function apiBaseURL() {
  const url = process.env.CALENDARSYNC_API_URL;
  if (!url) {
    throw new Error("missing CALENDARSYNC_API_URL");
  }
  return url;
}

function apiAudience() {
  return process.env.CALENDARSYNC_API_AUDIENCE ?? apiBaseURL();
}

function requiresIdentityToken(url: URL) {
  return url.hostname.endsWith(".run.app") || process.env.CALENDARSYNC_API_REQUIRE_ID_TOKEN === "true";
}

async function identityToken() {
  if (process.env.CALENDARSYNC_API_ID_TOKEN) {
    return process.env.CALENDARSYNC_API_ID_TOKEN;
  }
  const audience = encodeURIComponent(apiAudience());
  const res = await fetch(`${metadataIdentityURL}?audience=${audience}`, {
    headers: {
      "Metadata-Flavor": "Google"
    },
    cache: "no-store"
  });
  if (!res.ok) {
    throw new Error(`failed to mint API identity token: ${res.status}`);
  }
  return res.text();
}

async function headers(url: URL) {
  const session = await auth();
  if (!session?.user?.id || !session.user.email) {
    throw new Error("unauthenticated");
  }
  const result: Record<string, string> = {
    "Content-Type": "application/json",
    "X-User-ID": session.user.id,
    "X-User-Email": session.user.email
  };
  if (requiresIdentityToken(url)) {
    result.Authorization = `Bearer ${await identityToken()}`;
  }
  return result;
}

export async function apiFetch(path: string, init: RequestInit = {}) {
  const url = new URL(path, apiBaseURL());
  const mergedHeaders = {
    ...(init.headers ?? {}),
    ...(await headers(url))
  };
  return fetch(url.toString(), {
    ...init,
    headers: mergedHeaders,
    cache: "no-store"
  });
}

export async function apiGet(path: string) {
  const res = await apiFetch(path, { method: "GET" });
  return res.json();
}

export async function apiPost(path: string, body: unknown) {
  const res = await apiFetch(path, {
    method: "POST",
    body: JSON.stringify(body)
  });
  return res.json();
}

export async function apiPatch(path: string, body: unknown) {
  const res = await apiFetch(path, {
    method: "PATCH",
    body: JSON.stringify(body)
  });
  return res.json();
}

export async function apiDelete(path: string) {
  const res = await apiFetch(path, { method: "DELETE" });
  return res.json();
}
