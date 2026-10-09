# Data model and ownership
The core has no service database or migration system. History-v1 contains an initial signed64-bit register value and up to24 uniquely identified operation intervals. A write's value is its input; a completed read's value is its observed output. A pending read has no observed result. Times are centrally ordered integers, not synchronized wall clocks.

The checker owns the temporary (selected-operation mask, register value) search state. Its witness names a legal order; omitted pending writes are not asserted to have failed. Diagnostic reduction emits another history and its check/minimality metadata.

Disk-register-v1 contains explicit input events, reconstructed steps and a canonical codec digest. The model owns volatile/durable values. Crash discards unflushed state. A hash cannot prove authenticity; replay re-executes the model. The full planned raft-trace-v1 adds source/dependency/patch/entropy identities, event parents, incarnations, faults and client operations. It is a different schema, not an extension silently interpreted as the disk model.

See API_SPEC.md for exact JSON, limits, compatibility and error handling. All public fixtures are synthetic. Generated runs are local and ignored by Git; sanitize deliberately before publication.
