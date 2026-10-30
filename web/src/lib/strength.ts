export type Strength = {
  /** 0 (very guessable) to 4 (very unguessable) */
  score: 0 | 1 | 2 | 3 | 4
  warning: string
  suggestion: string
}

type Checker = (password: string, userInputs: string[]) => Strength

let checker: Promise<Checker> | undefined

// The dictionaries are large, so they are only fetched when the signup form
// first needs them, not on the login screen.
function load(): Promise<Checker> {
  checker ??= (async () => {
    const [{ ZxcvbnFactory }, common, en] = await Promise.all([
      import('@zxcvbn-ts/core'),
      import('@zxcvbn-ts/language-common'),
      import('@zxcvbn-ts/language-en'),
    ])
    const zxcvbn = new ZxcvbnFactory({
      translations: en.translations,
      graphs: common.adjacencyGraphs,
      dictionary: { ...common.dictionary, ...en.dictionary },
    })
    const run: Checker = (password, userInputs) => {
      const r = zxcvbn.check(password, userInputs)
      return {
        score: r.score,
        warning: r.feedback.warning ?? '',
        suggestion: r.feedback.suggestions[0] ?? '',
      }
    }
    return run
  })().catch((err) => {
    checker = undefined // allow a retry on the next keystroke
    throw err
  })
  return checker
}

/** Scores a password; userInputs (name, email) are treated as guessable words. */
export async function checkStrength(password: string, userInputs: string[] = []): Promise<Strength> {
  // zxcvbn matches whole words, so "Ada Lovelace" must become "Ada" and "Lovelace".
  const words = userInputs.flatMap((v) => v.split(/[\s@._+-]+/)).filter((w) => w.length > 1)
  return (await load())(password, words)
}
