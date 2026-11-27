package board

var tier1Slugs = map[string]struct{}{
	"openai": {}, "anthropic": {}, "cohere": {}, "xai": {}, "groq": {},
	"mistral": {}, "deepmind": {}, "jane-street": {}, "citadel": {},
	"two-sigma": {}, "hrt": {}, "jump-trading": {}, "optiver": {}, "imc": {},
	"stripe": {}, "databricks": {}, "palantir": {}, "scale-ai": {},
	"anduril": {}, "nvidia": {}, "cloudflare": {}, "snowflake": {},
	"spacex": {}, "tesla": {}, "waymo": {}, "netflix": {}, "airbnb": {},
	"figma": {}, "uber": {},
}

func isTier1Employer(slug string) bool {
	_, ok := tier1Slugs[slug]
	return ok
}

// IsTier1Employer reports whether the maintainer-curated prestige list contains
// the company slug.
func IsTier1Employer(slug string) bool {
	return isTier1Employer(slug)
}
