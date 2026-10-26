# FEBuilderGBA Visual Design System

Design completed: 2026-10-26

## Design brief

**Product**: FEBuilderGBA, a native Fire Emblem ROM editor
**Audience**: ROM hackers, game modders, Fire Emblem fan community
**Primary Function**: Edit Fire Emblem 8 (FE8U) character, class, and item data, plus raw hex values

## Color system

The palette is drawn from Fire Emblem's medieval fantasy and tactical RPG aesthetic:

```go
t.Accent = ui.Hex("#C44B2C")     // Burnt Sienna - iconic Fire Emblem flame/crest red
t.Success = ui.Hex("#4A7C59")    // Forest Green - growth and leveling system
t.Warning = ui.Hex("#D4941E")    // Gold - rare items and critical data
t.Danger = ui.Hex("#B3332D")     // Dark Red - errors and destructive operations
t.TextMuted = ui.Hex("#6B6B6B")  // Neutral grey for secondary information
```

### Color rationale

- **Burnt Sienna (#C44B2C).** The accent references the series' flame and crest imagery. This warm red-brown sits between the medieval setting (aged iron and rust) and the fantasy elements (fire magic, emblems).

- **Forest Green (#4A7C59).** Used for growth rates, stat gains, and progression states. Muted enough not to compete with the accent, distinct enough for success states.

- **Gold (#D4941E).** References legendary weapons and rare items. Used for warnings to draw attention without alarm.

- **Dark Red (#B3332D).** A deeper, more serious red than the accent, reserved for errors and destructive operations like data corruption or invalid ranges.

## Typography

The application uses MyGo's system defaults, plus a few font feature choices:

- **Tabular numbers** (`tnum`). Applied to hex IDs, addresses, and numeric values so columns align.
- **Monospace**. Used for hex dumps, ROM addresses, and byte displays.
- **Size hierarchy**:
  - 20px bold: Record names (character/class/item headers)
  - 13px bold: Section headers (abilities, combat, weapons)
  - 13px regular: Field labels and values
  - 12px regular: Hints, ranges, metadata

## Layout principles

1. **Data density.** ROM editing requires seeing many fields at once, so spacing is 3px.

2. **Three-zone structure**:
   - Left sidebar (330px): Searchable record list with hex IDs
   - Main editor: Field groups with inline editing
   - Bottom inspector: Active field details with +/- controls

3. **Responsive matrix.** The ability matrix (HP, Power, Speed, etc.) maintains grid alignment across base/growth/maximum/promotion columns, with weapon ranks flowing beside it on wide screens or wrapping on narrow ones.

4. **Minimal decoration.** 4px border radius. Borders appear only where they structure the layout (toolbar separation, table headers).

## Visual hierarchy

- **Accent color** highlights active selections, focused inputs, and primary actions
- **Muted text** keeps metadata (file size, record counts, field hints) in the background
- **Bold weight** distinguishes section headers and record names from field labels
- **Monospace + tabular numerals** create scannable columns of hex IDs and addresses

## Design principles

1. **Fire Emblem identity.** Colors reference the series' medieval fantasy and tactical heritage without mimicking any specific game's UI.
2. **Editing efficiency first.** Visual choices serve data legibility and editing efficiency over decoration.
3. **Clear state indication.** Modified documents, invalid inputs, and errors get immediate visual feedback.
4. **Density without clutter.** High information density comes from consistent spacing and typography, not from packing fields tighter.

## What this design avoids

- Generic AI defaults. No cream backgrounds, no teal/violet gradients, no Anthropic terracotta.
- SaaS card kit. No uniform rounded cards, no soft shadows, no glassmorphism.
- Unnecessary chrome. No ALL CAPS labels, no decorative numbered markers (01/02/03), no middle dots joining metadata.
- Template typography. No single-word accent colors in headlines, no arbitrary italic emphasis.

## Implementation notes

MyGo provides the theme structure; customization is limited to colors, spacing, and radius. Typography and layout work within these constraints, using font features (`tnum`, `monospace`) and semantic sizing to establish hierarchy.

The shared theme and window chrome live in `internal/components/chrome.go`. Record layouts, field controls and previews live in `internal/components/recordform`; `internal/pages` composes them with record navigation, batch editing and the hex editor.
