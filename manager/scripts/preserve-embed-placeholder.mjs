import { writeFile } from 'node:fs/promises'

await writeFile(new URL('../.output/public/.gitkeep', import.meta.url), '')
