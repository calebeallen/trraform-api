
export default {

	// The scheduled handler is invoked at the interval set in our wrangler.jsonc's
	// [[triggers]] configuration.
	async scheduled(event, env, ctx) {

        const SERVER_URL ="https://trraform-api-4a1d5fa5-8aa8-48ae-83b5-4771e43babf4.fly.dev"

        // update chunks and refresh leaderboard
        const triggers = [
            fetch(`${SERVER_URL}/cron-jobs/update-chunks`),
            fetch(`${SERVER_URL}/cron-jobs/refresh-leaderboard`)
        ]

        await Promise.all(triggers)

        return new Response();
		
	},
};
