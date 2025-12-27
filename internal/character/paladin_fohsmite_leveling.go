package character

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/mode"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
)

var _ context.LevelingCharacter = (*PalFohSLeveling)(nil)

const (
	paladinFohSmiteLevelingHfMaxAttacksLoop  = 10 // Maximum attack attempts before resetting
	paladinFohSmiteLevelingFohMinDistance    = 8
	paladinFohSmiteLevelingFohMaxDistance    = 15
	paladinFohSmiteLevelingHbMinDistance     = 6
	paladinFohSmiteLevelingHbMaxDistance     = 12
	paladinFohSmiteLevelingFohMaxAttacksLoop = 35              // Maximum attack attempts before resetting
	paladinFohSmiteLevelingCastingTimeout    = 3 * time.Second // Maximum time to wait for a cast to complete
)

type PalFohSLeveling struct {
	BaseCharacter
	lastCastTime time.Time
}

func (p PalFohSLeveling) useFohLogic() bool {
	return p.Data.PlayerUnit.Skills[skill.FistOfTheHeavens].Level > 0
}

func (p PalFohSLeveling) fohAttackOptions() ([]step.AttackOption, []step.AttackOption) {
	fohOpts := []step.AttackOption{
		step.StationaryDistance(paladinFohSmiteLevelingFohMinDistance, paladinFohSmiteLevelingFohMaxDistance),
		step.EnsureAura(skill.Salvation),
	}
	hbOpts := []step.AttackOption{
		step.StationaryDistance(paladinFohSmiteLevelingHbMinDistance, paladinFohSmiteLevelingHbMaxDistance),
		step.EnsureAura(skill.Salvation),
	}
	return fohOpts, hbOpts
}

func (p PalFohSLeveling) smiteAttackOptions() []step.AttackOption {
	return []step.AttackOption{
		step.Distance(1, 3),
		step.EnsureAura(skill.Salvation),
	}
}

func (p PalFohSLeveling) smiteTarget(targetID data.UnitID, opts []step.AttackOption) bool {
	step.SelectLeftSkill(skill.Smite)
	if err := step.PrimaryAttack(targetID, 1, false, opts...); err == nil {
		return true
	}
	return false
}

func (p PalFohSLeveling) holyFireAttackForLevel(targetID data.UnitID, level int) {
	switch {
	case level < 6:
		p.Logger.Debug("Using Might and Sacrifice")
		step.PrimaryAttack(targetID, 1, false, step.Distance(1, 3), step.EnsureAura(skill.Might))
	case level < 12:
		p.Logger.Debug("Using Holy Fire and Sacrifice")
		step.PrimaryAttack(targetID, 1, false, step.Distance(1, 3), step.EnsureAura(skill.HolyFire))
	default:
		p.Logger.Debug("Using Holy Fire and Zeal")
		step.PrimaryAttack(targetID, 1, false, step.Distance(1, 3), step.EnsureAura(skill.HolyFire))
	}
}

func (p PalFohSLeveling) hammerReposition() {
	p.Logger.Debug("Performing random movement to reposition.")
	p.PathFinder.RandomMovement()
	time.Sleep(time.Millisecond * 250)
}

func (p PalFohSLeveling) hammerPause() {
	time.Sleep(time.Millisecond * 250)
}

func (p PalFohSLeveling) attackHolyFireBoss(targetID data.UnitID, baseAttacks int, postHammer func()) {
	numOfAttacks := baseAttacks
	if p.Data.PlayerUnit.Skills[skill.BlessedHammer].Level > 0 {
		step.PrimaryAttack(targetID, numOfAttacks, false, step.Distance(2, 7), step.EnsureAura(skill.Concentration))
		if postHammer != nil {
			postHammer()
		}
		return
	}

	if p.Data.PlayerUnit.Skills[skill.Zeal].Level > 0 {
		numOfAttacks = 1 // Zeal is a multi-hit skill, 1 click is a sequence of attacks
	}
	step.PrimaryAttack(targetID, numOfAttacks, false, step.Distance(1, 3), step.EnsureAura(skill.HolyFire))
}

type bossLoopConfig struct {
	name             string
	npcID            npc.ID
	monsterType      data.MonsterType
	timeout          time.Duration
	baseAttacks      int
	approachDistance int
	errName          string
	postHammer       func()
}

func (p PalFohSLeveling) killBossLoop(cfg bossLoopConfig) error {
	if cfg.errName == "" {
		cfg.errName = cfg.name
	}
	if cfg.baseAttacks == 0 {
		cfg.baseAttacks = 5
	}

	p.Logger.Info(fmt.Sprintf("Starting %s kill sequence...", cfg.name))
	startTime := time.Now()
	fohOpts, hbOpts := p.fohAttackOptions()
	completedAttackLoops := 0

	for {
		boss, found := p.Data.Monsters.FindOne(cfg.npcID, cfg.monsterType)
		if !found {
			if time.Since(startTime) > cfg.timeout {
				p.Logger.Error(fmt.Sprintf("%s was not found, timeout reached.", cfg.name))
				return fmt.Errorf("%s not found within the time limit", cfg.errName)
			}
			time.Sleep(time.Second / 2)
			continue
		}

		if cfg.approachDistance > 0 {
			distance := p.PathFinder.DistanceFromMe(boss.Position)
			if distance > cfg.approachDistance {
				p.Logger.Debug(fmt.Sprintf("%s is too far away (%d), moving closer.", cfg.name, distance))
				step.MoveTo(boss.Position, step.WithIgnoreMonsters())
				continue
			}
		}

		if boss.Stats[stat.Life] <= 0 {
			p.Logger.Info(fmt.Sprintf("%s is dead.", cfg.name))
			return nil
		}

		if p.useFohLogic() {
			_ = p.attackBoss(boss.UnitID, fohOpts, hbOpts, &completedAttackLoops)
			continue
		}

		p.attackHolyFireBoss(boss.UnitID, cfg.baseAttacks, cfg.postHammer)
	}
}

func (p PalFohSLeveling) CheckKeyBindings() []skill.ID {
	requireKeybindings := []skill.ID{}
	missingKeybindings := []skill.ID{}

	for _, cskill := range requireKeybindings {
		if _, found := p.Data.KeyBindings.KeyBindingForSkill(cskill); !found {
			missingKeybindings = append(missingKeybindings, cskill)
		}
	}

	if len(missingKeybindings) > 0 {
		p.Logger.Debug("There are missing required key bindings.", slog.Any("Bindings", missingKeybindings))
	}

	return missingKeybindings
}

func (p PalFohSLeveling) BuffSkills() []skill.ID {
	skillsList := make([]skill.ID, 0)
	if _, found := p.Data.KeyBindings.KeyBindingForSkill(skill.HolyShield); found {
		skillsList = append(skillsList, skill.HolyShield)
	}
	return skillsList
}

func (p PalFohSLeveling) PreCTABuffSkills() []skill.ID {
	return []skill.ID{}
}

func (p PalFohSLeveling) ShouldResetSkills() bool {
	playerLevel, _ := p.Data.PlayerUnit.FindStat(stat.Level, 0)
	if playerLevel.Value == 60 && p.Data.PlayerUnit.Skills[skill.HolyFire].Level >= 2 {
		p.Logger.Info("Resetting skills: Level 60 and Holy Fire level >= 2")
		return true
	}

	return false
}

func (p PalFohSLeveling) SkillsToBind() (skill.ID, []skill.ID) {
	playerLevel, _ := p.Data.PlayerUnit.FindStat(stat.Level, 0)
	mainSkill := skill.AttackSkill
	skillBindings := []skill.ID{}

	if playerLevel.Value >= 6 {
		skillBindings = append(skillBindings, skill.Vigor)
	}

	if playerLevel.Value >= 24 {
		skillBindings = append(skillBindings, skill.BlessedHammer)
	}

	if p.Data.PlayerUnit.Skills[skill.HolyShield].Level > 0 {
		skillBindings = append(skillBindings, skill.HolyShield)
	}

	if p.Data.PlayerUnit.Skills[skill.BlessedHammer].Level > 0 && playerLevel.Value >= 18 {
		mainSkill = skill.BlessedHammer
	} else if playerLevel.Value < 6 {
		mainSkill = skill.Sacrifice
	} else if playerLevel.Value >= 6 && playerLevel.Value < 12 {
		mainSkill = skill.Sacrifice
	} else {
		mainSkill = skill.Zeal
	}

	if p.Data.PlayerUnit.Skills[skill.BattleCommand].Level > 0 {
		skillBindings = append(skillBindings, skill.BattleCommand)
	}

	if p.Data.PlayerUnit.Skills[skill.BattleOrders].Level > 0 {
		skillBindings = append(skillBindings, skill.BattleOrders)
	}

	_, found := p.Data.Inventory.Find(item.TomeOfTownPortal, item.LocationInventory)
	if found {
		skillBindings = append(skillBindings, skill.TomeOfTownPortal)
	}

	if p.Data.PlayerUnit.Skills[skill.Concentration].Level > 0 && playerLevel.Value >= 18 {
		skillBindings = append(skillBindings, skill.Concentration)
	} else {
		if playerLevel.Value < 6 {
			if _, found := p.Data.PlayerUnit.Skills[skill.Might]; found {
				skillBindings = append(skillBindings, skill.Might)
			}
		} else {
			if _, found := p.Data.PlayerUnit.Skills[skill.HolyFire]; found {
				skillBindings = append(skillBindings, skill.HolyFire)
			}
		}
	}

	p.Logger.Info("Skills bound", "mainSkill", mainSkill, "skillBindings", skillBindings)
	return mainSkill, skillBindings
}

func (p PalFohSLeveling) StatPoints() []context.StatAllocation {
	// Define target totals (including base stats)
	targets := []context.StatAllocation{
		{Stat: stat.Strength, Points: 40},  // +15 Str
		{Stat: stat.Dexterity, Points: 25}, // +5 Dex
		{Stat: stat.Vitality, Points: 30},  // +5 Vit
		{Stat: stat.Dexterity, Points: 30}, // +5 Dex
		{Stat: stat.Vitality, Points: 35},  // +5 Vit
		{Stat: stat.Dexterity, Points: 35}, // +5 Dex
		{Stat: stat.Vitality, Points: 40},  // +5 Vit
		{Stat: stat.Dexterity, Points: 40}, // +5 Dex
		{Stat: stat.Vitality, Points: 45},  // +5 Vit
		{Stat: stat.Dexterity, Points: 45}, // +5 Dex
		{Stat: stat.Vitality, Points: 50},  // +5 Vit
		{Stat: stat.Dexterity, Points: 50}, // +5 Dex
		{Stat: stat.Vitality, Points: 55},  // +5 Vit
		{Stat: stat.Dexterity, Points: 55}, // +5 Dex
		{Stat: stat.Vitality, Points: 60},  // +5 Vit
		{Stat: stat.Dexterity, Points: 60}, // +5 Dex

		{Stat: stat.Vitality, Points: 90}, // +30 Vit

		{Stat: stat.Strength, Points: 45},  // +5 Str
		{Stat: stat.Dexterity, Points: 65}, // +5 Dex
		{Stat: stat.Vitality, Points: 95},  // +5 Vit
		{Stat: stat.Strength, Points: 50},  // +5 Str
		{Stat: stat.Dexterity, Points: 70}, // +5 Dex
		{Stat: stat.Vitality, Points: 100}, // +5 Vit
		{Stat: stat.Strength, Points: 55},  // +5 Str
		{Stat: stat.Dexterity, Points: 75}, // +5 Dex
		{Stat: stat.Vitality, Points: 105}, // +5 Vit
		{Stat: stat.Strength, Points: 60},  // +5 Str
		{Stat: stat.Dexterity, Points: 80}, // +5 Dex
		{Stat: stat.Vitality, Points: 110}, // +5 Vit

		{Stat: stat.Dexterity, Points: 85},  // +5 Dex
		{Stat: stat.Vitality, Points: 115},  // +5 Vit
		{Stat: stat.Dexterity, Points: 90},  // +5 Dex
		{Stat: stat.Vitality, Points: 120},  // +5 Vit
		{Stat: stat.Dexterity, Points: 95},  // +5 Dex
		{Stat: stat.Vitality, Points: 125},  // +5 Vit
		{Stat: stat.Dexterity, Points: 100}, // +5 Dex
		{Stat: stat.Vitality, Points: 130},  // +5 Vit
		{Stat: stat.Dexterity, Points: 105}, // +5 Dex
		{Stat: stat.Vitality, Points: 135},  // +5 Vit

		{Stat: stat.Dexterity, Points: 125}, // +20 Dex
		{Stat: stat.Vitality, Points: 150},  // +15 Vit
		{Stat: stat.Strength, Points: 95},   // +35 Str
		{Stat: stat.Dexterity, Points: 145}, // +20 Dex

		{Stat: stat.Vitality, Points: 999}, // +xx Vit
		// 95 Str | 145 Dex | 999 Vit
	}

	return targets
}

func (p PalFohSLeveling) SkillPoints() []skill.ID {
	playerLevel, _ := p.Data.PlayerUnit.FindStat(stat.Level, 0)

	var skillSequence []skill.ID
	if playerLevel.Value < 60 {
		// Holy Fire build allocation for levels 1-59
		skillSequence = []skill.ID{
			skill.Might, skill.Sacrifice, skill.Smite, skill.ResistFire, skill.ResistFire, // 2-5 + Den of Evil

			skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, // 6-10
			skill.HolyFire, skill.Zeal, skill.HolyFire, skill.HolyFire, skill.HolyFire, // 11-15
			skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, // 16-19 + Radament
			skill.HolyFire, skill.HolyBolt, skill.Charge, skill.BlessedHammer, skill.HolyShield, // 20-24
			skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, // 25-29

			skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, // 30-32 + Izual
			skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, // 33-37
			skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, // 38-42
			skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.Salvation, skill.Salvation, // 43-46 + Den of Evil

			skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation, // 47-51
			skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation, // 52-55 + Radament
			skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation, // 56-60
		}
	} else {
		// FoH / Smite build allocation for levels 60+
		skillSequence = []skill.ID{
			skill.Prayer, skill.Defiance, skill.Cleansing, skill.Vigor, skill.Salvation,
			skill.Sacrifice, skill.Smite, skill.HolyBolt, skill.Zeal, skill.Charge,
			skill.Vengeance, skill.BlessedHammer, skill.Conversion, skill.HolyShield,

			skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens,
			skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens,
			skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens,
			skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens,

			skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt,
			skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt,
			skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt,
			skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt,

			skill.Smite, skill.Smite, skill.Smite, skill.Smite, skill.Smite,
			skill.Smite, skill.Smite, skill.Smite, skill.Smite, skill.Smite,
			skill.Smite, skill.Smite, skill.Smite, skill.Smite, skill.Smite,
			skill.Smite, skill.Smite, skill.Smite, skill.Smite,

			skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning,
			skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning,
			skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning,
			skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning,

			skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire,
			skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire,
			skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire,
			skill.ResistFire, skill.ResistFire, skill.ResistFire,
		}
	}

	return skillSequence
}

func (p PalFohSLeveling) GetAdditionalRunewords() []string {
	additionalRunewords := action.GetCastersCommonRunewords()
	additionalRunewords = append(additionalRunewords, "Steel")
	return additionalRunewords
}

func (p PalFohSLeveling) InitialCharacterConfigSetup() {

}

func (p PalFohSLeveling) AdjustCharacterConfig() {

}

func (p PalFohSLeveling) KillCountess() error {
	return p.killBoss(npc.DarkStalker, data.MonsterTypeSuperUnique)
}

func (p PalFohSLeveling) KillAndariel() error {
	return p.killBossLoop(bossLoopConfig{
		name:        "Andariel",
		npcID:       npc.Andariel,
		monsterType: data.MonsterTypeUnique,
		timeout:     time.Second * 160,
		postHammer:  p.hammerReposition,
	})
}

func (p PalFohSLeveling) KillSummoner() error {
	return p.killBoss(npc.Summoner, data.MonsterTypeUnique)
}

func (p PalFohSLeveling) KillDuriel() error {
	return p.killBossLoop(bossLoopConfig{
		name:        "Duriel",
		npcID:       npc.Duriel,
		monsterType: data.MonsterTypeUnique,
		timeout:     time.Second * 120,
		postHammer:  p.hammerReposition,
	})
}

func (p PalFohSLeveling) KillCouncil() error {
	return p.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		var councilMembers []data.Monster
		for _, m := range d.Monsters {
			if m.Name == npc.CouncilMember || m.Name == npc.CouncilMember2 || m.Name == npc.CouncilMember3 {
				councilMembers = append(councilMembers, m)
			}
		}

		// Order council members by distance
		sort.Slice(councilMembers, func(i, j int) bool {
			distanceI := p.PathFinder.DistanceFromMe(councilMembers[i].Position)
			distanceJ := p.PathFinder.DistanceFromMe(councilMembers[j].Position)

			return distanceI < distanceJ
		})

		if len(councilMembers) > 0 {
			p.Logger.Debug("Targeting Council member", "id", councilMembers[0].UnitID)
			return councilMembers[0].UnitID, true
		}

		p.Logger.Debug("No Council members found")
		return 0, false
	}, nil)
}

func (p PalFohSLeveling) KillMephisto() error {
	return p.killBossLoop(bossLoopConfig{
		name:        "Mephisto",
		npcID:       npc.Mephisto,
		monsterType: data.MonsterTypeUnique,
		timeout:     time.Second * 160,
		postHammer:  p.hammerReposition,
	})
}

func (p PalFohSLeveling) KillIzual() error {
	return p.killBossLoop(bossLoopConfig{
		name:             "Izual",
		npcID:            npc.Izual,
		monsterType:      data.MonsterTypeUnique,
		timeout:          time.Second * 120,
		approachDistance: 7,
		postHammer:       p.hammerReposition,
	})
}

func (p PalFohSLeveling) KillDiablo() error {
	return p.killBossLoop(bossLoopConfig{
		name:        "Diablo",
		npcID:       npc.Diablo,
		monsterType: data.MonsterTypeUnique,
		timeout:     time.Second * 120,
		baseAttacks: 10,
		errName:     "diablo",
		postHammer:  p.hammerPause,
	})
}

func (p PalFohSLeveling) KillPindle() error {
	return p.killBoss(npc.DefiledWarrior, data.MonsterTypeSuperUnique)
}

func (p PalFohSLeveling) KillNihlathak() error {
	return p.killBoss(npc.Nihlathak, data.MonsterTypeSuperUnique)
}

func (p PalFohSLeveling) KillAncients() error {
	originalBackToTownCfg := p.CharacterCfg.BackToTown
	p.CharacterCfg.BackToTown.NoHpPotions = false
	p.CharacterCfg.BackToTown.NoMpPotions = false
	p.CharacterCfg.BackToTown.EquipmentBroken = false
	p.CharacterCfg.BackToTown.MercDied = false

	for _, m := range p.Data.Monsters.Enemies(data.MonsterEliteFilter()) {
		foundMonster, found := p.Data.Monsters.FindOne(m.Name, data.MonsterTypeSuperUnique)
		if !found {
			continue
		}
		step.MoveTo(data.Position{X: 10062, Y: 12639}, step.WithIgnoreMonsters())

		p.killBoss(foundMonster.Name, data.MonsterTypeSuperUnique)

	}

	p.CharacterCfg.BackToTown = originalBackToTownCfg
	p.Logger.Info("Restored original back-to-town checks after Ancients fight.")
	return nil
}

func (p PalFohSLeveling) KillBaal() error {
	return p.killBossLoop(bossLoopConfig{
		name:        "Baal",
		npcID:       npc.BaalCrab,
		monsterType: data.MonsterTypeUnique,
		timeout:     time.Second * 600,
		postHammer:  p.hammerReposition,
	})
}

func (p PalFohSLeveling) ShouldIgnoreMonster(m data.Monster) bool {
	return false
}

func (p PalFohSLeveling) KillMonsterSequence(
	monsterSelector func(d game.Data) (data.UnitID, bool),
	skipOnImmunities []stat.Resist,
) error {
	if !p.useFohLogic() {
		const priorityMonsterSearchRange = 15
		completedAttackLoops := 0
		previousUnitID := 0

		priorityMonsters := []npc.ID{npc.FallenShaman, npc.MummyGenerator, npc.BaalSubjectMummy, npc.FetishShaman, npc.CarverShaman}

		for {
			context.Get().PauseIfNotPriority()

			var id data.UnitID
			var found bool

			var closestPriorityMonster data.Monster
			minDistance := -1

			for _, monsterNpcID := range priorityMonsters {
				for _, m := range p.Data.Monsters {
					if m.Name == monsterNpcID && m.Stats[stat.Life] > 0 {
						distance := p.PathFinder.DistanceFromMe(m.Position)
						if distance < priorityMonsterSearchRange {
							if minDistance == -1 || distance < minDistance {
								minDistance = distance
								closestPriorityMonster = m
							}
						}
					}
				}
			}

			if minDistance != -1 {
				id = closestPriorityMonster.UnitID
				found = true
				p.Logger.Debug("Priority monster found", "name", closestPriorityMonster.Name, "distance", minDistance)
			}

			if !found {
				id, found = monsterSelector(*p.Data)
			}

			if !found {
				return nil
			}

			if previousUnitID != int(id) {
				completedAttackLoops = 0
			}

			if !p.preBattleChecks(id, skipOnImmunities) {
				return nil
			}

			if completedAttackLoops >= paladinFohSmiteLevelingHfMaxAttacksLoop {
				return nil
			}

			monster, found := p.Data.Monsters.FindByID(id)
			if !found {
				p.Logger.Info("Monster not found", slog.String("monster", fmt.Sprintf("%v", monster)))
				return nil
			}

			playerLevel, _ := p.Data.PlayerUnit.FindStat(stat.Level, 0)
			p.holyFireAttackForLevel(id, playerLevel.Value)

			completedAttackLoops++
			previousUnitID = int(id)
		}
	} else {
		ctx := context.Get()
		lastRefresh := time.Now()
		completedAttackLoops := 0
		var currentTargetID data.UnitID
		useHolyBolt := false

		// Ensure we always return to FoH/Salvation when done
		defer func() {
			step.SelectLeftSkill(skill.FistOfTheHeavens)
			step.SelectRightSkill(skill.Salvation)
		}()

		fohOpts, hbOpts := p.fohAttackOptions()
		smiteOpts := p.smiteAttackOptions()

		// Initial target selection and analysis
		initialTargetAnalysis := func() (data.UnitID, bool, bool) {
			id, found := monsterSelector(*p.Data)
			if !found {
				return 0, false, false
			}

			// Count initial valid targets
			validTargets := 0
			//monstersInRange := make([]data.Monster, 0)
			monster, found := p.Data.Monsters.FindByID(id)
			if !found {
				return 0, false, false
			}

			for _, m := range ctx.Data.Monsters.Enemies() {
				if ctx.Data.AreaData.IsInside(m.Position) {
					dist := ctx.PathFinder.DistanceFromMe(m.Position)
					if dist <= paladinFohSmiteLevelingFohMaxDistance && dist >= paladinFohSmiteLevelingFohMinDistance && m.Stats[stat.Life] > 0 {
						validTargets++
						//monstersInRange = append(monstersInRange, m)
					}
				}
			}

			// Determine if we should use Holy Bolt
			// Only use Holy Bolt if it's a single target and it's immune to lightning
			shouldUseHB := validTargets == 1 && monster.IsImmune(stat.LightImmune) && monster.IsUndeadOrDemon()

			return id, true, shouldUseHB
		}

		for {
			context.Get().PauseIfNotPriority()

			// Refresh game data periodically
			if time.Since(lastRefresh) > time.Millisecond*100 {
				ctx.RefreshGameData()
				lastRefresh = time.Now()
			}

			ctx.PauseIfNotPriority()

			if completedAttackLoops >= paladinFohSmiteLevelingFohMaxAttacksLoop {
				return nil
			}

			// If we don't have a current target, get one and analyze the situation
			if currentTargetID == 0 {
				var found bool
				currentTargetID, found, useHolyBolt = initialTargetAnalysis()
				if !found {
					return nil
				}
			}

			// Verify our target still exists and is alive
			monster, found := p.Data.Monsters.FindByID(currentTargetID)
			if !found || monster.Stats[stat.Life] <= 0 {
				currentTargetID = 0 // Reset target
				continue
			}

			if !p.preBattleChecks(currentTargetID, skipOnImmunities) {
				return nil
			}

			// Ensure Salvation is active
			step.SelectRightSkill(skill.Salvation)

			if !monster.IsUndeadOrDemon() {
				if p.smiteTarget(currentTargetID, smiteOpts) {
					completedAttackLoops++
				}
				continue
			}

			// Cast appropriate skill
			if useHolyBolt {
				// Select Holy Bolt skill (uses packets if enabled, otherwise HID)
				step.SelectLeftSkill(skill.HolyBolt)
				if err := step.PrimaryAttack(currentTargetID, 1, true, hbOpts...); err == nil {
					if !p.waitForCastComplete() {
						continue
					}
					p.lastCastTime = time.Now()
					completedAttackLoops++
				}
			} else {
				// Select Fist of the Heavens skill (uses packets if enabled, otherwise HID)
				step.SelectLeftSkill(skill.FistOfTheHeavens)
				if err := step.PrimaryAttack(currentTargetID, 1, true, fohOpts...); err == nil {
					if !p.waitForCastComplete() {
						continue
					}
					p.lastCastTime = time.Now()
					completedAttackLoops++
				}
			}
		}
	}
}

func (p PalFohSLeveling) KillBossSequence(monsterSelector func(d game.Data) (data.UnitID, bool), skipOnImmunities []stat.Resist) error {
	// Specific Boss logic is only for FoH/Smite, fallback to Monster Sequence if in Holy Fire
	if !p.useFohLogic() {
		return p.KillMonsterSequence(monsterSelector, skipOnImmunities)
	}

	ctx := context.Get()
	lastRefresh := time.Now()
	completedAttackLoops := 0

	// Ensure we always return to FoH when done
	defer func() {
		step.SelectLeftSkill(skill.FistOfTheHeavens)
	}()

	fohOpts, hbOpts := p.fohAttackOptions()

	for {
		if time.Since(lastRefresh) > time.Millisecond*100 {
			ctx.RefreshGameData()
			lastRefresh = time.Now()
		}
		ctx.PauseIfNotPriority()
		if completedAttackLoops >= paladinFohSmiteLevelingFohMaxAttacksLoop {
			return nil
		}
		id, found := monsterSelector(*p.Data)
		if !found {
			return nil
		}
		if !p.preBattleChecks(id, skipOnImmunities) {
			return nil
		}
		monster, found := p.Data.Monsters.FindByID(id)
		if !found || monster.Stats[stat.Life] <= 0 {
			return nil
		}

		step.SelectRightSkill(skill.Salvation)

		if err := p.attackBoss(monster.UnitID, fohOpts, hbOpts, &completedAttackLoops); err == nil {
			continue
		}
	}
}

// waitForCastComplete waits until the character is no longer in casting animation
func (p PalFohSLeveling) waitForCastComplete() bool {
	ctx := context.Get()
	startTime := time.Now()

	for time.Since(startTime) < paladinFohSmiteLevelingCastingTimeout {
		ctx.RefreshGameData()

		// Check if we're no longer casting and enough time has passed since last cast
		if ctx.Data.PlayerUnit.Mode != mode.CastingSkill &&
			time.Since(p.lastCastTime) > 150*time.Millisecond { //150 for Foh but if we make that generic it would need tuning maybe from skill desc
			return true
		}

		time.Sleep(16 * time.Millisecond) // Small sleep to avoid hammering CPU
	}

	return false
}

func (p PalFohSLeveling) killBoss(npc npc.ID, t data.MonsterType) error {
	return p.KillBossSequence(func(d game.Data) (data.UnitID, bool) {
		m, found := d.Monsters.FindOne(npc, t)
		if !found || m.Stats[stat.Life] <= 0 {
			return 0, false
		}
		return m.UnitID, true
	}, nil)
}

func (p PalFohSLeveling) attackBoss(bossID data.UnitID, fohOpts, hbOpts []step.AttackOption, completedAttackLoops *int) error {
	monster, found := p.Data.Monsters.FindByID(bossID)
	if !found {
		return fmt.Errorf("boss not found")
	}

	if !monster.IsUndeadOrDemon() {
		smiteOpts := p.smiteAttackOptions()
		if p.smiteTarget(bossID, smiteOpts) {
			(*completedAttackLoops)++
		}
		return nil
	}

	// Cast FoH - use packet skill selection if enabled
	step.SelectLeftSkill(skill.FistOfTheHeavens)

	if err := step.PrimaryAttack(bossID, 1, true, fohOpts...); err == nil {
		// Wait for FoH cast to complete
		if !p.waitForCastComplete() {
			return fmt.Errorf("FoH cast timed out")
		}
		p.lastCastTime = time.Now()

		// Switch to Holy Bolt - use packet skill selection if enabled
		step.SelectLeftSkill(skill.HolyBolt)

		// Cast 2-3 Holy Bolts depending on FCR
		fcr, found := p.Data.PlayerUnit.FindStat(stat.FasterCastRate, 0)
		var hbCasts int
		if !found || fcr.Value < 75 {
			hbCasts = 2
		} else { // FCR >= 75
			hbCasts = 3
		}
		for i := 0; i < hbCasts; i++ {
			if err := step.PrimaryAttack(bossID, 1, true, hbOpts...); err == nil {
				if !p.waitForCastComplete() {
					return fmt.Errorf("Holy Bolt cast timed out")
				}
				p.lastCastTime = time.Now()
			}
		}

		(*completedAttackLoops)++
	}
	return nil
}
