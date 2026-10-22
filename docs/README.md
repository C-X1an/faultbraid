# FaultBraid / field guide

> **A practical route from a distributed failure to inspectable, source-bound evidence.**

[← Project overview](../README.md) · [Try the demo](demo/DEMO.md) · [Read the verification](../VERIFICATION_REPORT.md)

FaultBraid combines deterministic fault scheduling, an instrumented three-voter Raft register target, a bounded history checker, and executable counterexample reduction. This guide connects the concise overview to the deeper engineering material.

## Pick a starting point

| You want to… | Read this | What you get |
|:--|:--|:--|
| **See a failure** | [Five-minute reviewer demo](demo/DEMO.md) | Safe versus deliberately unsafe experiments; local HTML inspector |
| **Understand the system** | [Architecture](../ARCHITECTURE.md) | Scheduler, entropy, Raft integration, persistence and process boundaries |
| **Build against the CLI** | [API & command contract](../API_SPEC.md) | Schemas, verdicts, exit codes and replay identity |
| **Examine reproducibility** | [Trace codec](trace-codec.md) · [Data model](../DATA_MODEL.md) | Recorded entropy, source identity and native evidence |
| **Inspect real tests** | [Test matrix](../TEST_MATRIX.md) · [Evaluation](../EVALUATION.md) | Locked requirements, negative controls and comparison scopes |
| **Challenge a claim** | [Claims/evidence map](../CLAIMS_EVIDENCE.md) · [Verification](../VERIFICATION_REPORT.md) | Actual run identities, failures and known non-claims |
| **Assess trust boundaries** | [Threat model](../THREAT_MODEL.md) · [Limitations](../LIMITATIONS.md) | Assumptions and residual risks |
| **Contribute** | [Contribution guide](../CONTRIBUTING.md) | Test commands, evidence protocol and public-file review |

## Follow one counterexample

1. **Start with the deterministic demo:** from the repository root run `make bootstrap && make demo`. Open the generated `report.html` paths, rather than assuming a saved historical trace will replay against a different source build.
2. **Find the failed invariant:** compare the intentionally unsafe `unsafe-local-read` result with the safe variant. Check the operation intervals before interpreting the failure.
3. **Follow the cause:** in the inspector, read the reproduction identity, scenario actions, entropy records and causal events.
4. **Inspect the shrink:** compare the original and reduced executable scenario. The recorded A7 demonstration reduced [11 actions to five](../verification/release-a7/demo/reduced/reduction.json) while preserving the `INVALID` result.
5. **Question the evidence:** read the original run records and [independent assessment](../verification/reviews/final-a7-assessment.json). A bounded check or a recorded hash is not a general proof.

## System map

~~~mermaid
flowchart LR
    A["Scenario + recorded entropy"] --> B["Virtual scheduler"]
    B --> C["Instrumented 3-voter Raft"]
    C --> D["Client history"]
    D --> E["Bounded register checker"]
    C --> F["Source-bound trace"]
    F --> G["Re-execution / replay"]
    F --> H["Executable reduction"]
    H --> B
    E --> I["Static evidence inspector"]
    G --> I
~~~

**Look at the code:** [history checker](../internal/history/) · [Raft target](../internal/target/) · [virtual execution](../internal/scenario/) · [scenario reduction](../internal/campaign/) · [native-process lab](../internal/labreal/).

## Reading results correctly

| Result | Interpretation |
|:--|:--|
| `VALID` | A legal sequential explanation was constructed for this bounded history. |
| `INVALID` | Exhaustive admissible orders for the bounded history produced no legal explanation. |
| `INCONCLUSIVE` | Resource budget, cancellation or time prevented a definitive answer. |

All runs are finite. Source-bound A6 and A7 evidence retains its **original source, date and execution environment**; review [the evidence index](../verification/README.md) before comparing historical runs to the present tree. The deliberately unsafe read is a diagnostic variant, **not a vulnerability claim against upstream etcd/raft**. The native lab exercises actual processes but does not provide native replay or physical power-loss certification.

---

<p align="center"><strong>Inspect the record. Challenge the assumption. Re-run the failure.</strong><br><sub>FaultBraid / experimental systems-correctness engineering</sub></p>
