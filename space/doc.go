// Package space plans GBA ROM resource writes and relocations without changing
// the source bytes. It combines explicit references, resource ownership and
// bounded allocations into edits that the document can apply atomically.
//
// # Design and ownership
//
// Domain editors determine resource formats, encode payloads, choose references
// and declare protected regions. This package handles placement, pointer words,
// write conflicts and capacity. Pointer-shaped bytes are not reference evidence,
// and runs of zeroes or FF bytes are not evidence of reusable space.
//
// A [Builder] accumulates operations against one read-only snapshot. Each
// operation validates all its writes before adding any of them. Failure leaves
// previously accepted operations and the next allocation position unchanged.
// The builder borrows its source slice, copies accepted payloads and has no
// synchronization; the source and protection rules must remain unchanged while
// building. Use a separate builder for each editing operation.
//
// [WritePlan] is the detached planning result: destination information plus a
// rom.Edit containing byte writes and the resulting file size. Its fields are
// mutable, and it carries no document identity, revision or layout binding.
// [Builder.Plan] returns an independent copy, so later builder operations or
// changes to that copy cannot alter another preview. Destination is the bounding
// interval of allocated payloads; gaps inside it are not granted write access.
//
// The game/edit package owns the next stage. edit.NewCommit copies the rom.Edit
// into an immutable edit.Commit together with the original edit.Stamp and a
// history label. Commit.Apply checks the document and interpretation before
// delegating to rom.Document.ApplyEdit for an atomic revision check and write.
// Domain preview types retain their resource information alongside this pending
// commit. Planning and constructing a commit do not change document history.
// NewCommit freezes the supplied writes; it does not repeat resource checks if
// a caller changes the detached plan after validation.
//
// # Building and applying an edit
//
// Capture bytes and their stamp together through edit.Context.Capture before
// decoding or encoding. Derive payloads, selected pointers and the complete set
// of known claims from that input. CaptureBytes is for operations independent
// of interpretation. Drafts retain and check their original stamp; capturing
// another input must never make a stale draft valid again.
//
// In a domain editor, a replacement can be assembled as follows. Here ctx is an
// edit.Context; payload, selected and claims are domain results derived at the
// indicated point. selected is a slice of [PointerSite], and claims is a slice
// of [Claim]. The surrounding operation returns an error on failure.
//
//	input := ctx.Capture()
//	// Decode and validate payload, selected and claims from input.Data here.
//	builder := space.NewBuilder(input.Data, space.Protection{
//		Pointer: func(_ []byte, owner space.Owner, site int) error {
//			return space.CheckWrite(claims, owner, site)
//		},
//		Range: func(_ []byte, owner space.Owner, region space.Range) error {
//			return space.CheckRangeWrite(claims, owner, region)
//		},
//	})
//	if _, err := builder.Replace(payload, selected...); err != nil {
//		return err
//	}
//	writePlan := builder.Plan()
//	commit := edit.NewCommit(input.Stamp(), "Replace resource", writePlan.Edit)
//	// Keep commit with the domain preview; apply it after the preview is accepted.
//	if err := commit.Apply(ctx); err != nil {
//		return err
//	}
//
// A [PointerSite] contains an owner, the pointer's file offset and its expected
// complete uint32 word. ExpectedWord may be zero for a confirmed empty reference;
// it must match the source bytes exactly. The domain decides whether creating,
// replacing or clearing that reference is meaningful. Additional domain checks,
// including validation of a whole host record or script, remain the caller's
// responsibility. The protection callbacks can combine those checks with claims.
//
// # Composing operations
//
// [Builder.Append] appends a payload without changing existing references.
// [Builder.Replace] appends a payload and redirects only the selected pointers.
// Both preserve old payloads and independent references. [Builder.Repoint]
// takes a target file offset, so zero points to the start of the ROM. Use
// [Builder.ClearPointer] to write a null word. [Builder.Patch] replaces an
// existing byte range without moving the following bytes.
//
// Locations and [Range] bounds are file offsets, with half-open intervals.
// Pointer words use the GBA ROM base 0x08000000 and little-endian encoding.
// Pointer sites and appended payload starts are four-byte aligned. Allocation
// is bounded by rom.MaxSize; rom.Document fills expansion and alignment gaps
// with FF when applying the edit.
//
// Use one builder for all payloads and existing writes in a transaction.
// [Builder.AppendEncoded] and [Builder.ReplaceEncoded] resolve the next
// destination and pass its file offset to an encoder. The callback returns the
// complete payload, including any internal pointers. Encoding failure consumes
// no space; attempts to mutate the same builder inside the callback are rejected.
//
//	destination, err := builder.ReplaceEncoded(func(at int) ([]byte, error) {
//		return draft.Encode(at)
//	}, selected...)
//
// Repoint targets must lie in the source or the already planned expansion.
// Patches are limited to the original file, so encode appended payloads fully
// before adding them. Overlapping writes, including duplicate pointer sites,
// are rejected even when the requested bytes are identical. Combine edits to
// the same bytes in the domain draft before submitting them to the builder.
//
// # Claims and protection
//
// [Owner] identifies a resource; [Claim] describes its confirmed extent or
// pointer site. A site's Host identifies the record containing that pointer.
// [CheckWrite] checks a pointer update against overlapping claims, allowing the
// writer's own site and its declared host. [CheckRangeWrite] checks a payload
// update, allowing its owner's claims and pointer sites hosted by that owner.
// Owning a pointer does not grant permission to overwrite its host record.
//
// These checks allow unclaimed bytes; they do not establish a format or prove
// that every resource in the ROM has been discovered. Supply claims from all
// known resources, rather than only the resource being edited. [Protection]
// connects these checks and domain rules to the builder. A missing Pointer or
// Range callback rejects the corresponding existing-byte write; Append needs
// neither callback because it only creates new bytes.
//
// # Relocating existing resources
//
// [Builder.Relocate] copies a declared source range and fixes the selected
// [Reference] values, including offsets into the resource. Internal references
// are fixed only in the copy. The original source and unselected references
// remain intact; writes to that source before or after Relocate are rejected.
// [Reference] describes relocation offsets and external pointer ownership;
// [PointerSite] describes an expected word for replacement or repointing.
//
// Moves append by default. Reuse requires [FreeRange] entries with explicit
// evidence in Source; Move.Reserved protects other known regions from
// allocation. [ClaimRanges] converts claims to reserved intervals. Relocate
// shares allocation and overlap checks with all earlier builder operations.
// Each external reference passes Protection.Pointer with Reference.Owner.
// Reuse also passes Protection.Range with Move.Owner. The domain supplies
// complete source, reference and protection information.
//
// Errors use the stable [ErrRange], [ErrCapacity], [ErrReference] and [ErrClaim]
// codes, or preserve errors returned by encoding and protection callbacks. Callers use
// errors.Is and fault context rather than parsing error text.
package space
