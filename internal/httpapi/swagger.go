package httpapi

// @title wzap
// @version 1.0
// @description Standalone multi-instance WhatsApp gateway with authenticated REST commands and durable events. Normal REST JSON successes use {"data": ...}; standard errors use {"error": {"code", "message"}}. No-content responses, binary media, the raw Chatwoot webhook acknowledgement and Manager HTML/redirects have their own formats. Dual-auth API routes accept a valid wzap_session cookie or the apikey header. Global keys and admin sessions have administrative scope; user sessions are restricted to owned instances; instance keys reach only their own instance and cannot use collection/admin routes. Swagger 2.0 cannot declare cookie security: use an existing manager session cookie for session calls, or Authorize with an apikey for machine calls. The /auth/* endpoints use session cookies, never apikey authentication; login, logout, health probes, Swagger, Manager and the Chatwoot webhook load without an apikey.
// @BasePath /
// @securityDefinitions.apikey apikey
// @in header
// @name apikey
// @description Machine credential in the literal apikey header: a global key grants administrative scope, an instance key grants access only to its own instance. Dual-auth API routes also accept the wzap_session cookie (user ownership/admin scope); cookie security cannot be modeled in Swagger 2.0. /auth/* uses cookies rather than this header.
