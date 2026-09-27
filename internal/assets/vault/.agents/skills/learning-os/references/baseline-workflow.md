# Baseline workflow

Use this workflow when `learn status --json` reports `baseline.state = not_assessed`.

1. Ask whether the user wants a quick, standard, or deep baseline. Default to standard when they say only “start learning”.
2. Run `learn session start --kind baseline --depth <depth>` and verify the returned kind.
3. Ask one question at a time. Start with goals and familiar work, then use concrete scenarios to probe prerequisites. A standard baseline has 5–8 questions.
4. Keep the measurement clean: do not reveal an answer before the learner commits. “I don't know” is valid evidence.
5. Keep Curriculum Position unchanged throughout the baseline.
6. Build an assessment JSON following [assessment schema](assessment-schema.md). Every claim must quote a short, exact excerpt from the raw Conversation.
7. Pipe the JSON to `learn session end --assessment-file -`. Completion requires a saved assessment path and `baseline.state = assessed`.
8. Present existing knowledge, prerequisite gaps, possible misconceptions, and the recommended entry. The recommended entry must name an outline node (`recommended_entry.node`) taken from the confirmed outline, never a chapter title from memory. Update the position with `--node` only after the user confirms, and record any earlier entries the learner chooses to skip.

If the user explicitly skips baseline, start with `learn session start --kind lesson --skip-baseline` and state that personalization is limited until evidence is collected.
