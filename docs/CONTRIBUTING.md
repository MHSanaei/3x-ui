# Contributing to 3x-ui-docs

Thanks for helping improve the 3x-ui documentation and product site!

## Prerequisites

This project uses **[pnpm](https://pnpm.io)** (not npm — `package-lock.json` is
gitignored). Install dependencies and start the dev server:

```bash
pnpm install
pnpm dev        # http://localhost:3000
```

## Scripts

| Script           | Description                                           |
| ---------------- | ----------------------------------------------------- |
| `pnpm dev`       | Start the dev server                                  |
| `pnpm build`     | Production build                                      |
| `pnpm start`     | Serve the production build                            |
| `pnpm typecheck` | Generate MDX/route types and run `tsc --noEmit`       |
| `pnpm lint`      | ESLint (flat config)                                  |
| `pnpm format`    | Format with Prettier                                  |
| `pnpm test`      | Run unit tests (Vitest) for `lib/xray/*` pure logic   |
| `pnpm gen:api`   | Generate the API reference from `public/openapi.json` |

Before opening a pull request, please run `pnpm typecheck`, `pnpm lint`, and
`pnpm test` — these are the same checks that CI runs on every PR.

## License

By contributing, you agree that your contributions will be licensed under the
project's [GPL-3.0](./LICENSE) license.
