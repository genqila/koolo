package run

import (
	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/d2go/pkg/data/quest"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
)

type TalRashaTombs struct {
	ctx *context.Status
}

func NewTalRashaTombs() *TalRashaTombs {
	return &TalRashaTombs{
		ctx: context.Get(),
	}
}

func (a TalRashaTombs) Name() string {
	return string(config.TalRashaTombsRun)
}

func (a TalRashaTombs) CheckConditions(parameters *RunParameters) SequencerResult {
	if IsQuestRun(parameters) {
		return SequencerError
	}
	if !a.ctx.Data.Quests[quest.Act2TheSummoner].Completed() {
		return SequencerSkip
	}
	return SequencerOk
}

var talRashaTombs = []area.ID{area.TalRashasTomb1, area.TalRashasTomb2, area.TalRashasTomb3, area.TalRashasTomb4, area.TalRashasTomb5, area.TalRashasTomb6, area.TalRashasTomb7}

func (a TalRashaTombs) Run(parameters *RunParameters) error {
	// Iterate over all Tal Rasha Tombs.
	for _, tomb := range talRashaTombs {
		// Travel to the Canyon of the Magi waypoint.
		err := action.WayPoint(area.CanyonOfTheMagi)
		if err != nil {
			return err
		}

		// Enter the next tomb.
		if err = action.MoveToArea(tomb); err != nil {
			return err
		}

		// Open a TP if we're the leader
		action.OpenTPIfLeader()

		// Buff before we start
		action.Buff()

		// Find the Duriel tomb (orifice) or regular tomb (sparkly chest) special room.
		targetObject := data.Object{}
		for _, obj := range a.ctx.Data.Objects {
			if obj.Name == object.HoradricOrifice || obj.Name == object.SparklyChest {
				targetObject = obj
				break
			}
		}

		// Clear the Tomb until finding the special room.
		if targetObject.Name == 0 {
			a.ctx.Logger.Warn("Tal Rasha Tombs run: special room not found, exploring tomb")
			if err = action.ClearCurrentLevelEx(true, data.MonsterAnyFilter(), func() bool {
				// Find the Duriel tomb (orifice) or regular tomb (sparkly chest) special room.
				for _, obj := range a.ctx.Data.Objects {
					if obj.Name == object.HoradricOrifice || obj.Name == object.SparklyChest {
						targetObject = obj
						break
					}
				}
				// Room found, aborting the clear.
				if targetObject.Name != 0 {
					a.ctx.Logger.Warn("Tal Rasha Tombs run: special room found during exploration")
					return true
				}

				return false
			}); err != nil {
				return err
			}
			// Room was not found during the clear.
			if targetObject.Name == 0 {
				a.ctx.Logger.Warn("Tal Rasha Tombs run: special room not found after exploration")
			}
		}

		// Move to the special room.
		if targetObject.Name != 0 {
			if err := action.MoveToCoords(targetObject.Position); err != nil {
				return err
			}
		}

		// Return to town before moving to the next tomb.
		if err = action.ReturnTown(); err != nil {
			return err
		}

		// Allow early stop for leveling sequences that cap the desired level.
		if parameters != nil && parameters.SequenceSettings != nil && parameters.SequenceSettings.MaxLevel != nil {
			ctx := context.Get()
			if lvl, found := ctx.Data.PlayerUnit.FindStat(stat.Level, 0); found && lvl.Value > *parameters.SequenceSettings.MaxLevel {
				a.ctx.Logger.Info("Tal Rasha Tombs run: interrupted due to max level reached")
				return nil
			}
		}
	}

	// All tombs completed.
	return nil
}
