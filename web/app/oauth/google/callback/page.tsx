import { auth } from "@/auth";
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

  const url = new URL(`${process.env.CALENDARSYNC_API_URL}/api/connections/google/callback`);
  url.searchParams.set("code", code);
  url.searchParams.set("state", state);
  await fetch(url.toString(), {
    method: "GET",
    headers: {
      "X-User-ID": session.user.id,
      "X-User-Email": session.user.email
    },
    cache: "no-store"
  });
  redirect("/");
}
