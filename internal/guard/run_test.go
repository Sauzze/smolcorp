package guard

import "testing"

// T307c: yarn, bun and pnpm "run" hand the program on, like "yarn vercel" does.
func TestRunHandsTheProgramOn(t *testing.T) {
	mustRefuse(t, "run <bin>", []string{
		"yarn run vercel deploy", "bun run vercel deploy", "bun run netlify deploy", "bun run wrangler deploy",
		"bun run eas build", "bun --bun run vercel deploy", "bun --cwd x run vercel deploy", "bun --filter x run vercel",
		"bun --cwd x x vercel", "pnpm run vercel deploy", "pnpm run-script vercel deploy", "yarn run-script vercel deploy",
		"bun --cwd=x run vercel deploy",
	})
	mustRun(t, "run <script>", []string{
		"yarn run build", "bun run dev", "pnpm run test", "bun --cwd x run build", "bun --filter x run lint",
		"yarn run render", "bun run fly", "pnpm run-script render", "npm run build", "bun run",
	})
}
