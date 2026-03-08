import { auth } from "@/auth";

async function headers() {
  const session = await auth();
  if (!session?.user?.id || !session.user.email) {
    throw new Error("unauthenticated");
  }
  return {
    "Content-Type": "application/json",
    "X-User-ID": session.user.id,
    "X-User-Email": session.user.email
  };
}

export async function apiGet(path: string) {
  const res = await fetch(`${process.env.CALENDARSYNC_API_URL}${path}`, {
    method: "GET",
    headers: await headers(),
    cache: "no-store"
  });
  return res.json();
}

export async function apiPost(path: string, body: unknown) {
  const res = await fetch(`${process.env.CALENDARSYNC_API_URL}${path}`, {
    method: "POST",
    headers: await headers(),
    body: JSON.stringify(body),
    cache: "no-store"
  });
  return res.json();
}

export async function apiPatch(path: string, body: unknown) {
  const res = await fetch(`${process.env.CALENDARSYNC_API_URL}${path}`, {
    method: "PATCH",
    headers: await headers(),
    body: JSON.stringify(body),
    cache: "no-store"
  });
  return res.json();
}

export async function apiDelete(path: string) {
  const res = await fetch(`${process.env.CALENDARSYNC_API_URL}${path}`, {
    method: "DELETE",
    headers: await headers(),
    cache: "no-store"
  });
  return res.json();
}
