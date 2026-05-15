# NEXT — Cursor for the active milestone

The active goal is the **first unchecked** `### G…` heading in [PLAN.md](./PLAN.md)'s **Active execution sequence** section. Discharge it to its `Measurable` criterion (per PLAN.md's "SMART contract for every goal" section). When green, run `mdedit toggle -s "<G>" PLAN.md` to tick its checkbox — the next session's cursor advances automatically because it reads the new first-unchecked heading.
