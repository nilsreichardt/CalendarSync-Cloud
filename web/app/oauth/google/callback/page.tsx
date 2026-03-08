import { auth } from "@/auth";
import { apiFetch } from "@/lib/api";
import { redirect } from "next/navigation";

type Props = {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

export default async function GoogleCallbackPage({ searchParams }: Props) {
  const session = await auth();
  if (!session?.user?.id || !session.user.email) {
    redirect("/");
  }

  const params = await searchParams;
  const code = typeof params.code === "string" ? params.code : "";
  const state = typeof params.state === "string" ? params.state : "";
  if (!code || !state) {
    redirect("/");
  }

  const url = new URL("/api/connections/google/callback", process.env.CALENDARSYNC_API_URL);
  url.searchParams.set("code", code);
  url.searchParams.set("state", state);
  await apiFetch(`${url.pathname}?${url.searchParams.toString()}`, { method: "GET" });
  redirect("/");
}
