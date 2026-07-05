// Local-only config used when serving the frontend via docker-compose.
// docker-compose.yml volume-mounts this file over js/config.js in the
// frontend container so the real (production-pointed) config.js never needs
// to change. The anon key here is Supabase CLI's fixed public demo key -
// the same for every local `supabase start`, not a secret.
export const SUPABASE_URL = "http://localhost:54321";
export const SUPABASE_ANON_KEY =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZS1kZW1vIiwicm9sZSI6ImFub24iLCJleHAiOjE5ODM4MTI5OTZ9.CRXP1A7WOeoJeXxjNni43kdQwgnWNReilDMblYTn_I0";
export const API_BASE_URL = "http://localhost:8080";
