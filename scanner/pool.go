package scanner

// DefaultSNICandidates is a curated list of domain names that are:
//   - Hosted on Cloudflare or other major CDNs that the bypass targets
//   - Unlikely to be on any national blocklist (benign SaaS / tooling)
//   - Stable over time (core infrastructure, not trending domains)
//
// The scanner probes each against the user's chosen target IP. SNIs that
// work populate a profile's sni_pool for bypass rotation.
//
// Keep this list small (~60 entries). A large list makes scans slow without
// improving the result: the top-ranked handful is what goes into the pool.
var DefaultSNICandidates = []string{
	// Known-good Cloudflare-fronted, commonly used by legit services
	"auth.vercel.com",
	"cdn.segment.io",
	"static.cloudflareinsights.com",
	"ajax.cloudflare.com",
	"challenges.cloudflare.com",
	"cf-assets.www.cloudflare.com",
	"www.cloudflare.com",
	"developers.cloudflare.com",

	// Developer/SaaS tools — rarely blocked
	"api.github.com",
	"raw.githubusercontent.com",
	"gist.github.com",
	"github.githubassets.com",
	"objects.githubusercontent.com",
	"api.stripe.com",
	"js.stripe.com",
	"checkout.stripe.com",
	"api.notion.com",
	"www.notion.so",
	"api.linear.app",
	"cdn.linear.app",
	"api.figma.com",
	"www.figma.com",
	"cdn.sanity.io",
	"www.sanity.io",
	"api.openai.com",
	"chat.openai.com",
	"api.anthropic.com",

	// CDN front door domains
	"cdn.jsdelivr.net",
	"unpkg.com",
	"cdnjs.cloudflare.com",
	"fonts.googleapis.com",
	"fonts.gstatic.com",

	// Hosting / platform
	"vercel.app",
	"nextjs.org",
	"netlify.app",
	"app.netlify.com",
	"fly.io",
	"render.com",
	"railway.app",
	"pages.dev",
	"workers.dev",

	// Analytics / tracking (low-risk)
	"plausible.io",
	"app.posthog.com",
	"eu.posthog.com",
	"api.amplitude.com",
	"api.mixpanel.com",
	"cdn.amplitude.com",

	// Developer docs / static content
	"developer.mozilla.org",
	"stackoverflow.com",
	"www.npmjs.com",
	"registry.npmjs.org",
	"pypi.org",
	"files.pythonhosted.org",

	// Video / large-bandwidth endpoints
	"www.cloudflare.tv",
	"stream.cloudflare.com",
}

// DefaultCloudflareIPs is a small set of Cloudflare anycast IPs that
// actually serve TLS on :443 (verified handshakes). We avoid the .0 of each
// /24 because those are network-address placeholders, not edge servers.
//
// These IPs were picked from:
//   - 1.1.1.1 / 1.0.0.1 (Cloudflare DNS, universally reachable)
//   - Sampled /32s from Cloudflare's public /12 ranges (cloudflare.com/ips)
//
// Expanded lists belong elsewhere so they can be refreshed without touching
// the scanner.
var DefaultCloudflareIPs = []string{
	"1.1.1.1",
	"1.0.0.1",
	"104.16.132.229",
	"104.17.2.81",
	"104.18.24.17",
	"104.19.90.17",
	"104.20.64.1",
	"104.21.2.17",
	"104.22.38.200",
	"104.26.10.39",
	"172.64.41.3",
	"172.65.250.78",
	"172.66.40.17",
	"172.67.160.80",
	"188.114.96.7",
	"188.114.97.7",
	"188.114.98.7",
	"188.114.99.7",
	"162.159.140.229", // discord / workers tail
	"162.159.61.3",
}
