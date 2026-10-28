package game

import (
	"bytes"
	"encoding/binary"
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

// MaxMovementSteps is the native REDA buffer capacity, not the count-byte limit.
const MaxMovementSteps = 16

const movementResource = "deploy_movement"

const (
	MoveX        = "move_x"
	MoveY        = "move_y"
	MoveConfig   = "move_config"
	MoveSyncUnit = "move_sync_unit"
	MoveSyncStep = "move_sync_step"
	MoveDelay    = "move_delay"
)

// MovementStep is one FE8 REDA: an absolute destination, movement configuration,
// synchronization with another moving character, and a delay before this step.
// Raw retains the complete eight bytes, including unused bits.
type MovementStep struct {
	Position                          image.Point
	Config, SyncUnit, SyncStep, Delay int
	Raw                               []byte
}

func (d *DeploymentDraft) SupportsPaths() bool { return d.group.Stride == 20 }

func (d *DeploymentDraft) MovementFields() []DeploymentField {
	if !d.SupportsPaths() {
		return nil
	}
	return []DeploymentField{
		{ID: MoveX, Max: 63, offset: 0, width: 2},
		{ID: MoveY, Max: 63, offset: 0, width: 2, shift: 6},
		{ID: MoveConfig, Max: 63, offset: 0, width: 4, shift: 12},
		{ID: MoveSyncUnit, Max: 255, offset: 3, width: 1},
		{ID: MoveSyncStep, Max: 0xFFFF, offset: 4, width: 2},
		{ID: MoveDelay, Max: 0xFFFF, offset: 6, width: 2},
	}
}

func (d *DeploymentDraft) Path(unit int) []MovementStep {
	if !d.SupportsPaths() || unit < 0 || unit >= len(d.records) {
		return nil
	}
	raw := d.records[unit].movement
	result := make([]MovementStep, len(raw)/8)
	for index := range result {
		data := raw[index*8 : index*8+8]
		position := u16(data, 0)
		result[index] = MovementStep{
			Position: image.Pt(int(position&63), int((position>>6)&63)),
			Config:   int((u32(data, 0) >> 12) & 63),
			SyncUnit: int(data[3]),
			SyncStep: int(
				u16(data, 4),
			),
			Delay: int(u16(data, 6)),
			Raw:   bytes.Clone(data),
		}
	}
	return result
}

func (d *DeploymentDraft) pathRecord(unit int) (*deploymentRecord, error) {
	if !d.SupportsPaths() || unit < 0 || unit >= len(d.records) {
		return nil, ErrField
	}
	return &d.records[unit], nil
}

func (d *DeploymentDraft) PathValue(unit, step int, id string) (int64, error) {
	record, err := d.pathRecord(unit)
	if err != nil || step < 0 || step >= len(record.movement)/8 {
		return 0, ErrField
	}
	for _, field := range d.MovementFields() {
		if field.ID == id {
			return (deploymentRecord{data: record.movement[step*8 : step*8+8]}).value(
				field,
			), nil
		}
	}
	return 0, ErrField
}

func (d *DeploymentDraft) SetPathField(
	unit, step int,
	id string,
	value int64,
) (err error) {
	context := fault.Context{
		Resource:   movementResource,
		RecordID:   new(unit),
		ResourceID: new(step),
		Field:      id,
		Value:      new(value),
	}
	defer func() { err = fault.Wrap(err, "game.change_deployment_path", context) }()
	record, err := d.pathRecord(unit)
	if err != nil || step < 0 || step >= len(record.movement)/8 {
		return ErrField
	}
	for _, field := range d.MovementFields() {
		if field.ID != id {
			continue
		}
		if value < field.Min || value > field.Max {
			context.Bounds = &fault.Bounds{Min: field.Min, Max: field.Max}
			return ErrValue
		}
		data := record.movement[step*8 : step*8+8]
		if (deploymentRecord{data: data}).value(field) == value {
			return nil
		}
		writeDeploymentValue(data, field.offset, field, value)
		d.movementChanged(record)
		return nil
	}
	return ErrField
}

func movementStep(position image.Point) []byte {
	result := make([]byte, 8)
	binary.LittleEndian.PutUint16(
		result,
		uint16(position.X&63)|uint16(position.Y&63)<<6,
	) // #nosec G115 -- Coordinates are masked to native six-bit fields.
	result[4], result[5] = 0xFF, 0xFF
	return result
}

func (d *DeploymentDraft) movementChanged(record *deploymentRecord) {
	record.data[7] = byte(
		len(record.movement) / 8,
	) // #nosec G115 -- Native readers bound paths to the original count byte.
	record.movementEdited = true
	if len(record.movement) == 0 {
		clear(record.data[8:12])
	}
	d.generation++
}

func (d *DeploymentDraft) AddPathStep(unit int) error {
	record, err := d.pathRecord(unit)
	if err != nil {
		return err
	}
	if len(record.movement)/8 >= MaxMovementSteps {
		return ErrValue
	}
	record.movement = append(
		record.movement,
		movementStep(record.unit().End)...)
	d.movementChanged(record)
	return nil
}

func (d *DeploymentDraft) DuplicatePathStep(unit, step int) error {
	record, err := d.pathRecord(unit)
	if err != nil {
		return err
	}
	if step < 0 || step >= len(record.movement)/8 {
		return ErrField
	}
	if len(record.movement)/8 >= MaxMovementSteps {
		return ErrValue
	}
	data := bytes.Clone(record.movement[step*8 : step*8+8])
	record.movement = slices.Insert(record.movement, (step+1)*8, data...)
	d.movementChanged(record)
	return nil
}

func (d *DeploymentDraft) RemovePathStep(unit, step int) error {
	record, err := d.pathRecord(unit)
	if err != nil {
		return err
	}
	if step < 0 || step >= len(record.movement)/8 {
		return ErrField
	}
	record.movement = slices.Delete(record.movement, step*8, step*8+8)
	d.movementChanged(record)
	return nil
}

func (d *DeploymentDraft) MovePathStep(unit, from, to int) error {
	record, err := d.pathRecord(unit)
	if err != nil {
		return err
	}
	count := len(record.movement) / 8
	if from < 0 || from >= count || to < 0 || to >= count {
		return ErrField
	}
	if from == to {
		return nil
	}
	raw := bytes.Clone(record.movement[from*8 : from*8+8])
	if from < to {
		copy(record.movement[from*8:to*8], record.movement[from*8+8:to*8+8])
	} else {
		copy(record.movement[to*8+8:from*8+8], record.movement[to*8:from*8])
	}
	copy(record.movement[to*8:to*8+8], raw)
	d.movementChanged(record)
	return nil
}

func (d *DeploymentDraft) validatePaths() error {
	for index, record := range d.records {
		if record.movementEdited && len(record.movement) > MaxMovementSteps*8 {
			return fault.Wrap(
				ErrValue,
				"game.validate_deployment_path",
				fault.Context{
					Resource: movementResource,
					RecordID: new(index),
					Value: new(
						int64(len(record.movement) / 8),
					),
					Bounds: &fault.Bounds{Min: 0, Max: MaxMovementSteps},
				},
			)
		}
	}
	return nil
}
