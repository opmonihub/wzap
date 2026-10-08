package manager

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// DevHandler forwards the manager namespace to a validated development
// origin. The caller parses the configured origin once during composition.
func DevHandler(target *url.URL) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			// Route to the dev origin without joining or stripping any path.
			r.Out.URL.Scheme = target.Scheme
			r.Out.URL.Host = target.Host
			// ReverseProxy cleans unparsable parameters before Rewrite. Nuxt
			// must receive the original query, including asset cache keys.
			r.Out.URL.RawQuery = r.In.URL.RawQuery
			r.Out.Host = r.In.Host
			r.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			// Transport errors may contain internal addresses or request data.
			http.Error(w, "manager development server is unavailable", http.StatusServiceUnavailable)
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manager" {
			location := "/manager/"
			if r.URL.RawQuery != "" || r.URL.ForceQuery {
				location += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, location, http.StatusMovedPermanently)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}
