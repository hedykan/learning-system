# Review workflow

The Runtime schedules spaced reviews (1, 2, 4, 7, 15, 30, 60 days). You run them.

## When

- `learn next --json` returns `action: review_due`, or `learn review --json` lists due concepts.
- Do due reviews at the start of a session, before new material, unless the learner asks otherwise. Review a concept at most once per session.

## How

1. Ask one retrieval question about the concept without hints, in a scenario the learner has not seen for it. Do not show the concept note first.
2. Let the learner answer fully. Do not rescue them with the answer.
3. Judge the answer:
   - `recalled`: the core idea came back correctly without help.
   - `partial`: the direction is right but a key part is missing or needed a hint.
   - `forgotten`: the idea did not come back or came back wrong.
4. Give brief feedback. For `partial` or `forgotten`, repair the gap with the smallest useful explanation, then ask a quick check.
5. Submit the outcome in the next checkpoint:

   ```json
   {"review_results": [{"id": "r1", "concept": "tail-latency", "outcome": "recalled",
     "action_turn": "t0012", "evidence": [{"turn": "t0013", "quote": "exact learner words"}]}]}
   ```

   `action_turn` is your question; evidence is the learner's answer after it. A recalled answer in a later session is also retrieval evidence: add a `retrieval` event, and a `stable` state update when the concept already had a developing state.

Never mention intervals, schedules, or the Runtime to the learner. Say “先回忆一下上次学的……” instead.
