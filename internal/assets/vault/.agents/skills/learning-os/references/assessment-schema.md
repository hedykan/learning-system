# Baseline assessment schema

Send this JSON to `learn session end --assessment-file -` for a baseline:

```json
{
  "curriculum": "curriculum-id",
  "depth": "standard",
  "learning_goals": [{"claim": "...", "evidence": "exact learner excerpt"}],
    "existing_knowledge": [{"claim": "...", "evidence": "exact learner excerpt", "concepts": ["concept-id"]}],
  "prerequisite_gaps": [{"claim": "...", "evidence": "exact learner excerpt"}],
  "possible_misconceptions": [{"claim": "...", "evidence": "exact learner excerpt"}],
  "familiar_vocabulary": ["..."],
  "unknown_vocabulary": ["..."],
  "recommended_entry": {
    "book": "curriculum-id",
    "chapter": "...",
    "section": "...",
    "current_concept": "...",
    "last_completed": "",
    "next_textbook_step": "",
    "detour": null
  },
    "recommendation_reason": "...",
  "next_probe": "...",
  "goal_card": {
    "outcome": {"text": "what they want to be able to do", "evidence": "exact learner excerpt"},
    "context": {"text": "where they will use it", "evidence": "..."},
    "background": {"text": "what they already know", "evidence": "..."},
    "constraints": {"text": "time, depth, preferences", "evidence": "..."},
    "success_criteria": {"text": "how they will know", "evidence": "..."},
    "focus": [{"id": "multi-tenancy", "text": "a concrete point they care about", "evidence": "..."}]
  }
}
```

`concepts` (optional, on any finding) are kebab-case concept ids, the same ids outline entries declare; they let the intake evidence reach the outline. `goal_card` is optional; only `outcome` is required inside it, and every field needs the learner's exact words.

Evidence is an exact, short excerpt from the raw learner turn. Do not cite assistant explanations as proof of learner knowledge.
