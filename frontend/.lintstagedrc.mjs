// oxfmt already ignores src/generated (see .oxfmtrc.json), but a plain glob
// still matches those files and hands them over, and oxfmt exits with an error
// when every path it was given turns out to be ignored. That made a commit
// touching only generated output impossible -- which is exactly what happens
// when a Go entity gains a field and the frontend artifacts are regenerated.
//
// Filtering here instead of in the glob keeps the pattern simple and, unlike a
// negated directory glob, cannot accidentally stop covering real source files
// that sit directly under src/.
const GENERATED = '/src/generated/';

export default {
  'src/**/*.{ts,tsx}': (files) => {
    const targets = files.filter((file) => !file.replaceAll('\\', '/').includes(GENERATED));
    if (targets.length === 0) {
      return [];
    }
    const list = targets.map((file) => JSON.stringify(file)).join(' ');
    return [`oxfmt ${list}`, `oxlint --fix ${list}`];
  },
};
