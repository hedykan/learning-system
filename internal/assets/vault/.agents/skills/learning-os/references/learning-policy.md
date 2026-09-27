# Guided Socratic learning policy

Use this default cycle for one concept:

1. **Elicit**: ask for the learner's current model in a familiar scenario.
2. **Predict**: require a concrete outcome before explaining it.
3. **Reason**: ask why the prediction should hold.
4. **Challenge**: change one condition or offer a counterexample.
5. **Revise**: ask the learner to state the improved model in their own words.
6. **Transfer**: test the model in a different scenario.

Each step must use observable Conversation evidence. Recognition or repetition is not mastery; `stable` needs retrieval or transfer evidence from a later session than the first explanation.

## Vocabulary budget

- Introduce at most two unestablished terms in one teaching turn.
- Start with an example, then name the abstraction.
- For each term, give one concrete example, one boundary from a neighboring concept, and one lightweight check.
- When the learner asks what a foundational term means, record a prerequisite gap, provide the smallest useful explanation, and return to the Socratic cycle.

## Strategy switch

After two attempts with no observable cognitive change, switch representation: example, analogy, diagram, counterexample, learner action, or prerequisite repair. Record each attempt as a `strategy_attempt`; the new one names the old one in `replaces` with a `switch_reason`. The Runtime stops recommending a strategy that failed twice in a row on the same concept.

## Curriculum fidelity

The curriculum decides what the mainline is; the learner model decides how to teach it. Do not reorder the textbook because of learner evidence. For a prerequisite gap:

```bash
learn curriculum detour start --topic "<prerequisite>" --concept <concept-id> --reason "<gap evidence>" --return-condition "<observable check>"
learn curriculum detour end --outcome completed --learned "<what the learner can now do>"
```

`detour end` returns the position to the recorded point. When `learn next` answers `return_to_mainline`, close the detour and resume there.
