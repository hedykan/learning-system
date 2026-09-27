# Baseline assessment schema

Send this JSON to `learn session end --assessment-file -` for a baseline:

```json
{
  "curriculum": "curriculum-id",
  "depth": "standard",
  "learning_goals": [{"claim": "...", "evidence": "exact learner excerpt"}],
  "existing_knowledge": [{"claim": "...", "evidence": "exact learner excerpt"}],
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
  "next_probe": "..."
}
```

Evidence is an exact, short excerpt from the raw learner turn. Do not cite assistant explanations as proof of learner knowledge.
