package chapter

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
)

func (s *Editor) mapChangeReferences(
	draft *maps.ChangesDraft,
	uses []maps.Use,
) maps.ChangeReferences {
	var report maps.ChangeReferences
	chapters := make(map[int]bool)
	for _, use := range uses {
		if use.Updated {
			chapters[use.Chapter] = true
		}
	}
	warn := func(chapter, address int, err error) {
		for _, old := range report.Warnings {
			if old.Chapter == chapter &&
				fault.CodeOf(old.Err) == fault.CodeOf(err) {
				return
			}
		}
		report.Warnings = append(
			report.Warnings,
			maps.ChangeWarning{Chapter: chapter, Address: address, Err: err},
		)
	}
	remaining := 65536
	for _, use := range uses {
		if !use.Updated {
			continue
		}
		entries, err := s.Events.ScriptEntries(use.Chapter)
		if err != nil {
			warn(use.Chapter, 0, err)
			continue
		}
		seen := make(map[int]bool)
		for _, entry := range entries {
			if entry.Err != nil {
				warn(use.Chapter, entry.Pointer, entry.Err)
				continue
			}
			if entry.Address == 0 || seen[entry.Address] {
				continue
			}
			seen[entry.Address] = true
			program, err := s.Events.Script(use.Chapter, entry.Pointer)
			if err != nil {
				warn(use.Chapter, entry.Pointer, err)
				continue
			}
			if !program.Complete {
				warn(use.Chapter, entry.Pointer, event.ErrMapChangeContext)
			}
			for _, trace := range program.MapTrace() {
				remaining--
				if remaining < 0 {
					warn(
						use.Chapter,
						trace.Address,
						event.ErrMapChangeLimit,
					)
					return report
				}
				if trace.Uncertain {
					warn(
						use.Chapter,
						trace.Address,
						event.ErrMapChangeContext,
					)
				}
				if !trace.IsTarget {
					continue
				}
				if trace.Chapter >= 0 && !chapters[trace.Chapter] {
					continue
				}
				operands, err := trace.Targets, trace.Err
				if err != nil {
					warn(use.Chapter, trace.Address, err)
					continue
				}
				for _, operand := range operands {
					if len(report.References) >= 4096 {
						warn(
							use.Chapter,
							trace.Address,
							event.ErrMapChangeLimit,
						)
						return report
					}
					ref := draft.CompareTarget(operand)
					ref.Chapter, ref.ScriptChapter = trace.Chapter, use.Chapter
					ref.Pointer, ref.Address, ref.Instruction = entry.Pointer, trace.Address, trace.Instruction
					if operand.ByPosition && trace.RuntimePositions &&
						(ref.Before < 0 || ref.After < 0) {
						warn(
							use.Chapter,
							trace.Address,
							event.ErrMapChangeContext,
						)
					}
					report.References = append(report.References, ref)
				}
			}
		}
	}
	return report
}
