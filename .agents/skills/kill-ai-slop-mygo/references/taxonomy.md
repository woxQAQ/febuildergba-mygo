# MyGo design tells

A tell is a reason to inspect context. Intentional visual identity, native widget
conventions and domain content take precedence over these heuristics. M01–M12
are scanner IDs local to this skill, not the web skill's numbering.

## Automated candidates

| ID  | What to inspect                                                         | Useful correction                                                                    | Legitimate uses to preserve                                                          |
| --- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| M01 | Gradients on every panel, blurred decoration, glowing status surfaces   | Use a theme background and establish grouping with alignment or a divider            | Color ramps, actual image effects, visualization, intentional branding               |
| M02 | Literal colors in UI chrome that bypass appearance-aware theme roles    | Reuse the appropriate theme role; centralize a repeated product choice               | Swatches, sprites, chart series, faction colors, a deliberate theme definition       |
| M03 | Pill-shaped panels and controls with large repeated corner radii        | Follow the existing radius scale; remove unnecessary inner surfaces                  | Circular indicators, avatars, intentional segmented controls                         |
| M04 | Wide diffuse shadows on ordinary editor regions                         | Reserve elevation for overlays; separate ordinary regions with space or a hairline   | Floating dialogs, menus, drag previews, image effects                                |
| M05 | A greeting or full sentence at display size displacing the working data | Short task title, normal-size explanatory text, data visible immediately             | Presentation mode, an accessibility setting, a genuinely important metric            |
| M06 | Decorative serif families on field labels and dense values              | Keep the current readable UI font and its language fallbacks                         | Document previews, typography editors, a deliberate editorial product                |
| M07 | A thick colored left stripe repeated on every note or record            | Place a short message and relevant action beside the affected field                  | A selected row, diff marker, or a consequential warning with text                    |
| M08 | Badges for ordinary headings, every value or decorative “new” claims    | Use plain labels; retain badges only where state/count matters                       | Unsaved state, version, pending count, filters, actual release status                |
| M09 | Large padding at each nested level or one large gap everywhere          | Tight inside a related group; larger between unrelated groups                        | Touch input, low-vision mode, breathing room around a complex preview                |
| M10 | Generic claims, dramatic contrasts and promotional empty states         | Name the object and action: “Open a project”, “3 files could not be read”            | Quoted/user-authored content, accurate domain terminology                            |
| M11 | Emoji on every button or heading                                        | Remove repetition; use clear text and existing meaningful icons                      | Emoji input/pickers, actual document content, a deliberate status convention         |
| M12 | `Text` or `Box` made clickable to imitate a button                      | Check focus, keyboard activation, disabled state and accessible name; prefer widgets | Canvas hit regions, working base-widget composition, deliberate non-command gestures |

The thresholds are search aids. They are not a design scale or a reason to shrink
all text, remove all icons, replace the system font, or make every view gray.

## Manual review

These questions require reading composition and behavior, often with a rendered
view. The scanner does not claim to detect them.

**Nested cards and broken corners.** Follow `Children` callbacks and shared
helpers. Multiple backgrounds, borders, radii and padding layers can turn an
editor into a narrow stack of boxes. Keep one surface per meaningful region;
independent previews and embedded editors may need their own. Check clipping and
stroke ownership at rounded corners. A text scan cannot prove their geometry.

**Comparison and density.** Values compared by the user belong in aligned rows or
columns. Replacing a capability matrix with one card per stat makes comparison
harder. Keep IDs, units, unknown values and source distinctions readable. Use
right alignment and tabular numerals for numeric columns, monospace for addresses
and bytes. Read-only values still need body-text contrast; “not editable” does
not mean “unimportant.” Preserve virtualization for long lists.

**Flat hierarchy or decorative hierarchy.** Repeated uppercase kickers, redundant
section numbers, highlighted words, excessive bolding and uniform gaps can hide
relationships. Dense desktop interfaces can use a narrow font-size range when
alignment, weight and spacing make hierarchy clear. Do not enforce web display
ratios. Never remove real record IDs or order numbers as decorative ordinals.

**Color semantics.** An entire warning region need not be yellow, but a warning
must remain recognizable. Keep explanatory text and the recovery action. Review
light and dark appearances, focus rings, selected rows and disabled controls.
Color alone must not carry the state; domain colors such as map factions are
content, not arbitrary branding.

**Actions and status.** Multiple primary buttons compete when they address the
same decision; separate independent tasks may each justify one. Identify the
primary action per active task. Distinguish editing a draft, previewing a change,
applying it and saving a document. Remove fake “live” dots and invented metrics;
keep dirty indicators, real counts and current job status. Explain disabled
commands when the reason is not obvious.

**Empty, loading and error states.** Show what is missing and the next available
action. Avoid giant mascots, welcome slogans and permanent “ready” cards taking
up the editing area. Keep blocking errors, conflict details, incomplete-check
warnings and recovery controls visible. Optional format background can move to
nearby help; data-loss consequences cannot depend on hovering.

**Motion and painting.** Inspect custom `Painter`/drawing callbacks, animations,
repeated invalidation and hover transforms. Motion should explain a state change,
not make static panels breathe. Keep working loading feedback. Watch a custom
spinner complete a turn and verify its center; prefer an existing widget. Pixel
art, path overlays and palette canvases are functional content, not decorative
illustrations to replace.

**Localization and resizing.** Compare the supported languages, long names, large
IDs and unknown-value labels. Check the supported minimum window size and zoom.
Use local scrolling, wrapping or a collapsible optional section where the task
calls for it. Do not shrink labels until they are illegible or hide an important
action to make a screenshot fit.

## Relationship to the web skill

This retains the original skill's focus on unnecessary gradients, display type,
generic copy, emoji, repeated callouts, badges, excessive surfaces and motion.
Web-specific CSS recipes, SVG artwork replacement, marketing typography rules
and blanket objections to default fonts or semantic palettes do not transfer
directly. Native widget consistency, data density and behavior are part of the
review, alongside visual choices.
