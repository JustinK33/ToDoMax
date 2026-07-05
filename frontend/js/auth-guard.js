import { supabase } from "./supabase-client.js";

// Redirects to login.html if there's no active session. Resolves with the
// session when authenticated, for pages that need it (e.g. to attach a JWT).
export async function requireSession() {
  const { data } = await supabase.auth.getSession();
  if (!data.session) {
    window.location.replace("login.html");
    return null;
  }
  return data.session;
}
