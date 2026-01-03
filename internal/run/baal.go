package run

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/d2go/pkg/data/quest"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/data/state"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/utils"
)

// Positions adapted from kolbot baal.js
var baalThroneMainPos = data.Position{X: 15095, Y: 5042}
var blizzPos = data.Position{X: 15094, Y: 5027}
var hammerPos = data.Position{X: 15094, Y: 5029}
var throneCenter = data.Position{X: 15093, Y: 5029}
var forwardPos = data.Position{X: 15116, Y: 5026}

type Baal struct {
	ctx                *context.Status
	clearMonsterFilter data.MonsterFilter // Used to clear area (basically TZ)
	preAtkLast         time.Time
	decoyLast          time.Time
}

func NewBaal(clearMonsterFilter data.MonsterFilter) *Baal {
	return &Baal{
		ctx:                context.Get(),
		clearMonsterFilter: clearMonsterFilter,
	}
}

func (s Baal) Name() string {
	return string(config.BaalRun)
}

func (a Baal) CheckConditions(parameters *RunParameters) SequencerResult {
	farmingRun := IsFarmingRun(parameters)
	if !a.ctx.Data.Quests[quest.Act5RiteOfPassage].Completed() {
		if farmingRun {
			return SequencerSkip
		}
		return SequencerStop
	}
	questCompleted := a.ctx.Data.Quests[quest.Act5EveOfDestruction].Completed()
	if (farmingRun && !questCompleted) || (!farmingRun && questCompleted) {
		return SequencerSkip
	}
	return SequencerOk
}
func (s *Baal) Run(parameters *RunParameters) error {
	// Set filter
	filter := data.MonsterAnyFilter()
	if s.ctx.CharacterCfg.Game.Baal.OnlyElites {
		filter = data.MonsterEliteFilter()
	}
	if s.clearMonsterFilter != nil {
		filter = s.clearMonsterFilter
	}

	err := action.WayPoint(area.TheWorldStoneKeepLevel2)
	if err != nil {
		return err
	}

	if s.ctx.CharacterCfg.Game.Baal.ClearFloors || s.clearMonsterFilter != nil {
		action.ClearCurrentLevel(false, filter)
	}

	err = action.MoveToArea(area.TheWorldStoneKeepLevel3)
	if err != nil {
		return err
	}

	if s.ctx.CharacterCfg.Game.Baal.ClearFloors || s.clearMonsterFilter != nil {
		action.ClearCurrentLevel(false, filter)
	}

	err = action.MoveToArea(area.ThroneOfDestruction)
	if err != nil {
		return err
	}
	err = action.MoveToCoords(baalThroneMainPos)
	if err != nil {
		return err
	}
	if s.checkForSoulsOrDolls() {
		return errors.New("souls or dolls detected, skipping")
	}

	// Let's move to a safe area and open the portal in companion mode
	if s.ctx.CharacterCfg.Companion.Leader {
		action.MoveToCoords(data.Position{
			X: 15116,
			Y: 5071,
		})
		action.OpenTPIfLeader()
	}

	err = action.ClearAreaAroundPlayer(50, data.MonsterAnyFilter())
	if err != nil {
		return err
	}

	// Force rebuff before waves
	action.Buff()

	// Come back to previous position
	err = action.MoveToCoords(baalThroneMainPos)
	if err != nil {
		return err
	}

	// Process waves until Baal leaves throne
	s.ctx.Logger.Info("Starting Baal waves...")
	waveTimeout := time.Now().Add(7 * time.Minute)

	lastWaveDetected := false
	isWaitingForPortal := false
	_, isLevelingChar := s.ctx.Char.(context.LevelingCharacter)

	for !s.hasBaalLeftThrone() && time.Now().Before(waveTimeout) {
		s.ctx.PauseIfNotPriority()
		s.ctx.RefreshGameData()

		// Detect last wave for logging
		if _, found := s.ctx.Data.Monsters.FindOne(npc.BaalsMinion, data.MonsterTypeMinion); found {
			if !lastWaveDetected {
				s.ctx.Logger.Info("Last wave (Baal's Minion) detected")
				lastWaveDetected = true
			}
		} else if lastWaveDetected {
			if !s.ctx.CharacterCfg.Game.Baal.KillBaal && !isLevelingChar {
				s.ctx.Logger.Info("Waves cleared, skipping Baal kill (Fast Exit).")
				return nil
			}

			if !isWaitingForPortal {
				s.ctx.Logger.Info("Waves cleared, moving to portal position to wait...")
				action.MoveToCoords(data.Position{X: 15090, Y: 5008})
				isWaitingForPortal = true
			}

			utils.Sleep(500)
			continue
		}

		if !isWaitingForPortal {
			action.ClearAreaAroundPosition(baalThroneMainPos, 50, data.MonsterAnyFilter())
			if s.ctx.Data.PlayerUnit.Skills[skill.BlessedHammer].Level > 1 {
				// Hammerdin should preAttack from monster spawn area
				action.MoveToCoords(hammerPos)
			} else {
				action.MoveToCoords(baalThroneMainPos)
			}

			s.preAttackBaalWaves()
		}

		utils.Sleep(500) // Prevent excessive checking
	}

	if !s.hasBaalLeftThrone() {
		return errors.New("baal waves timeout - portal never appeared")
	}

	// Baal has entered the chamber
	s.ctx.Logger.Info("Baal has entered the Worldstone Chamber")

	// Kill Baal Logic
	if s.ctx.CharacterCfg.Game.Baal.KillBaal || isLevelingChar {
		action.Buff()

		s.ctx.Logger.Info("Waiting for Baal portal...")
		var baalPortal data.Object
		found := false

		for i := 0; i < 15; i++ {
			baalPortal, found = s.ctx.Data.Objects.FindOne(object.BaalsPortal)
			if found {
				break
			}
			utils.Sleep(300)
		}

		if !found {
			return errors.New("baal portal not found after waves completed")
		}

		s.ctx.Logger.Info("Entering Baal portal...")

		// Enter portal
		err = action.InteractObject(baalPortal, func() bool {
			return s.ctx.Data.PlayerUnit.Area == area.TheWorldstoneChamber
		})

		// Verify entry
		if s.ctx.Data.PlayerUnit.Area == area.TheWorldstoneChamber {
			s.ctx.Logger.Info("Successfully entered Worldstone Chamber")
		} else if err != nil {
			return fmt.Errorf("failed to enter baal portal: %w", err)
		}

		// Move to Baal (may fail due to tentacles)
		s.ctx.Logger.Info("Moving to Baal...")
		moveErr := action.MoveToCoords(data.Position{X: 15136, Y: 5943})
		if moveErr != nil {
			if strings.Contains(moveErr.Error(), "path could not be calculated") {
				s.ctx.Logger.Info("Path blocked by tentacles, attacking from current position")
			} else {
				s.ctx.Logger.Warn("Failed to move to Baal", "error", moveErr)
			}
		}

		return s.ctx.Char.KillBaal()
	}

	return nil
}

// hasBaalLeftThrone checks if Baal has left the throne and entered the Worldstone Chamber
func (s *Baal) hasBaalLeftThrone() bool {
	_, found := s.ctx.Data.Monsters.FindOne(npc.BaalThrone, data.MonsterTypeNone)
	return !found
}

func (s Baal) checkForSoulsOrDolls() bool {
	var npcIds []npc.ID

	if s.ctx.CharacterCfg.Game.Baal.DollQuit {
		npcIds = append(npcIds, npc.UndeadStygianDoll2, npc.UndeadSoulKiller2)
	}
	if s.ctx.CharacterCfg.Game.Baal.SoulQuit {
		npcIds = append(npcIds, npc.BlackSoul2, npc.BurningSoul2)
	}

	for _, id := range npcIds {
		if _, found := s.ctx.Data.Monsters.FindOne(id, data.MonsterTypeNone); found {
			return true
		}
	}

	return false
}

func (s *Baal) preAttackBaalWaves() {
	// Simple global cooldown between preattacks to avoid spam
	const preAtkCooldown = 300 * time.Millisecond
	if !s.preAtkLast.IsZero() && time.Since(s.preAtkLast) < preAtkCooldown {
		return
	}

	// Sorceress
	if s.ctx.Data.PlayerUnit.Skills[skill.Blizzard].Level > 0 {
		step.CastAtPosition(skill.Blizzard, false, true, blizzPos)
		s.preAtkLast = time.Now()
		return
	}

	if s.ctx.Data.PlayerUnit.Skills[skill.Meteor].Level > 0 {
		step.CastAtPosition(skill.Meteor, false, true, blizzPos)
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.FrozenOrb].Level > 0 {
		step.CastAtPosition(skill.FrozenOrb, false, true, blizzPos)
		s.preAtkLast = time.Now()
		return
	}

	// Paladin
	if s.ctx.Data.PlayerUnit.Skills[skill.BlessedHammer].Level > 1 {
		if kb, found := s.ctx.Data.KeyBindings.KeyBindingForSkill(skill.Concentration); found {
			s.ctx.HID.PressKeyBinding(kb)
		}
		step.CastAtPosition(skill.BlessedHammer, true, true, hammerPos)
		s.preAtkLast = time.Now()
		if !s.ctx.Data.PlayerUnit.States.HasState(state.Meditation) {
			if kb, found := s.ctx.Data.KeyBindings.KeyBindingForSkill(skill.Meditation); found {
				s.ctx.HID.PressKeyBinding(kb)
			}
		} else if !s.ctx.Data.PlayerUnit.States.HasState(state.Cleansing) {
			if kb, found := s.ctx.Data.KeyBindings.KeyBindingForSkill(skill.Cleansing); found {
				s.ctx.HID.PressKeyBinding(kb)
			}
		}
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.HolyBolt].Level > 1 {
		if !s.ctx.Data.PlayerUnit.States.HasState(state.Cleansing) {
			if kb, found := s.ctx.Data.KeyBindings.KeyBindingForSkill(skill.Cleansing); found {
				s.ctx.HID.PressKeyBinding(kb)
			}
		}
		step.CastAtPosition(skill.HolyBolt, true, true, hammerPos)
		s.preAtkLast = time.Now()
		return
	}

	// Amazon
	if s.ctx.Data.PlayerUnit.Skills[skill.Decoy].Level > 0 {
		const decoyCooldown = 10 * time.Second
		if s.decoyLast.IsZero() || time.Since(s.decoyLast) > decoyCooldown {
			decoyPos := data.Position{X: 15092, Y: 5028}
			step.CastAtPosition(skill.Decoy, false, false, decoyPos)
			s.decoyLast = time.Now()
			s.preAtkLast = time.Now()
			return
		}
	}

	// Necromancer
	if s.ctx.Data.PlayerUnit.Skills[skill.PoisonNova].Level > 0 {
		step.CastAtPosition(skill.PoisonNova, false, true, s.ctx.Data.PlayerUnit.Position)
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.DimVision].Level > 0 {
		step.CastAtPosition(skill.DimVision, false, true, blizzPos)
		s.preAtkLast = time.Now()
		return
	}

	// Druid:
	if s.ctx.Data.PlayerUnit.Skills[skill.Tornado].Level > 0 {
		step.CastAtPosition(skill.Tornado, false, true, throneCenter)
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.Fissure].Level > 0 {
		step.CastAtPosition(skill.Fissure, false, true, forwardPos)
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.Volcano].Level > 0 {
		step.CastAtPosition(skill.Volcano, false, true, forwardPos)
		s.preAtkLast = time.Now()
		return
	}

	// Assassin:
	if s.ctx.Data.PlayerUnit.Skills[skill.LightningSentry].Level > 0 {
		for i := 0; i < 3; i++ {
			step.CastAtPosition(skill.LightningSentry, false, true, throneCenter)
			utils.Sleep(80)
		}
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.DeathSentry].Level > 0 {
		for i := 0; i < 2; i++ {
			step.CastAtPosition(skill.DeathSentry, false, true, throneCenter)
			utils.Sleep(80)
		}
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.ShockWeb].Level > 0 {
		step.CastAtPosition(skill.ShockWeb, false, true, throneCenter)
		s.preAtkLast = time.Now()
		return
	}
}
