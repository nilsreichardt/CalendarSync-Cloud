import NextAuth from "next-auth";
import Google from "next-auth/providers/google";

function normalizeEmail(email?: string | null) {
  return email?.trim().toLowerCase() ?? "";
}

export const { handlers, signIn, signOut, auth } = NextAuth({
  providers: [
    Google({
      clientId: process.env.AUTH_GOOGLE_ID!,
      clientSecret: process.env.AUTH_GOOGLE_SECRET!
    })
  ],
  callbacks: {
    async jwt({ token, account, profile }) {
      if (account?.provider === "google" && profile && "sub" in profile && typeof profile.sub === "string") {
        token.sub = profile.sub;
      }
      if (typeof token.email === "string") {
        token.email = normalizeEmail(token.email);
      }
      return token;
    },
    async session({ session, token }) {
      if (session.user) {
        session.user.id = typeof token.sub === "string" ? token.sub : "";
        session.user.email = normalizeEmail(session.user.email ?? token.email);
      }
      return session;
    }
  }
});
