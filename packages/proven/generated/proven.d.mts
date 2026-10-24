// Types for proven.mjs, which `pnpm proofs:build` compiles from PROOF.bend.
// Keys are "<module path>:<def>"; constructors cross as { $: "Name", ...fields }.
declare const proven: Record<string, (...args: never[]) => unknown>;
export default proven;
