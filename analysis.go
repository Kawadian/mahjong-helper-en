package main

import (
	"fmt"
	"strings"

	"github.com/EndlessCheng/mahjong-helper/util"
	"github.com/EndlessCheng/mahjong-helper/util/model"
	"github.com/fatih/color"
)

func simpleBestDiscardTile(playerInfo *model.PlayerInfo) int {
	shanten, results14, incShantenResults14 := util.CalculateShantenWithImproves14(playerInfo)
	bestAttackDiscardTile := -1
	if len(results14) > 0 {
		bestAttackDiscardTile = results14[0].DiscardTile
	} else if len(incShantenResults14) > 0 {
		bestAttackDiscardTile = incShantenResults14[0].DiscardTile
	} else {
		return -1
	}
	if shanten == 1 && len(playerInfo.DiscardTiles) < 9 && len(results14) > 0 && len(incShantenResults14) > 0 && !playerInfo.IsNaki() { // 鸣牌时的向听倒退暂不考虑
		if results14[0].Result13.Waits.AllCount() < 9 && results14[0].Result13.MixedWaitsScore < incShantenResults14[0].Result13.MixedWaitsScore {
			bestAttackDiscardTile = incShantenResults14[0].DiscardTile
		}
	}
	return bestAttackDiscardTile
}

// TODO: 重构至 model
func humanMeld(meld model.Meld) string {
	humanMeld := util.TilesToStr(meld.Tiles)
	if meld.MeldType == model.MeldTypeAnkan {
		return strings.ToUpper(humanMeld)
	}
	return humanMeld
}
func humanHands(playerInfo *model.PlayerInfo) string {
	humanHands := util.Tiles34ToStr(playerInfo.HandTiles34)
	if len(playerInfo.Melds) > 0 {
		humanHands += " " + model.SepMeld
		for i := len(playerInfo.Melds) - 1; i >= 0; i-- {
			humanHands += " " + humanMeld(playerInfo.Melds[i])
		}
	}
	return humanHands
}

func analysisPlayerWithRisk(playerInfo *model.PlayerInfo, mixedRiskTable riskTable) error {
	// Hand tiles
	humanTiles := humanHands(playerInfo)
	fmt.Println(humanTiles)
	fmt.Println(strings.Repeat("=", len(humanTiles)))

	countOfTiles := util.CountOfTiles34(playerInfo.HandTiles34)
	switch countOfTiles % 3 {
	case 1:
		result := util.CalculateShantenWithImproves13(playerInfo)
		fmt.Println("Current " + util.NumberToChineseShanten(result.Shanten) + ":")
		r := &analysisResult{
			discardTile34:  -1,
			result13:       result,
			mixedRiskTable: mixedRiskTable,
		}
		r.printWaitsWithImproves13_oneRow()
	case 2:
		// Analyze hand tiles
		shanten, results14, incShantenResults14 := util.CalculateShantenWithImproves14(playerInfo)

		// Prompt information
		if shanten == -1 {
			color.HiRed("【Winning Hand】")
		} else if shanten == 0 {
			if len(results14) > 0 {
				r13 := results14[0].Result13
				if r13.RiichiPoint > 0 && r13.FuritenRate == 0 && r13.DamaPoint >= 5200 && r13.DamaWaits.AllCount() == r13.Waits.AllCount() {
					color.HiGreen("Sufficient points for silent tenpai: aim for tenpai silently, aim for points with riichi")
				}
				// When the round income is similar, prompt: similar round income, pursue win rate by discarding xx, pursue points by discarding xx
			}
		} else if shanten == 1 {
			// In early to mid rounds, when closed, remind of backward to shanten 2
			if len(playerInfo.DiscardTiles) < 9 && !playerInfo.IsNaki() {
				alertBackwardToShanten2(results14, incShantenResults14)
			}
		}

		// TODO: Near the end of the round, prompt which player's discard is at the bottom of the river

		// Discard analysis result
		printResults14WithRisk(results14, mixedRiskTable)
		printResults14WithRisk(incShantenResults14, mixedRiskTable)
	default:
		err := fmt.Errorf("Invalid parameters: %d tiles", countOfTiles)
		if debugMode {
			panic(err)
		}
		return err
	}

	fmt.Println()
	return nil
}

// Analyze meld
// playerInfo: Player information
// targetTile34: Opponent's discarded tile
// isRedFive: Whether the discarded tile is a red five
// allowChi: Whether chi is allowed
// mixedRiskTable: Risk table
func analysisMeld(playerInfo *model.PlayerInfo, targetTile34 int, isRedFive bool, allowChi bool, mixedRiskTable riskTable) error {
	if handsCount := util.CountOfTiles34(playerInfo.HandTiles34); handsCount%3 != 1 {
		return fmt.Errorf("Invalid hand: %d tiles %v", handsCount, playerInfo.HandTiles34)
	}
	// Original hand analysis
	result := util.CalculateShantenWithImproves13(playerInfo)
	// Meld analysis
	shanten, results14, incShantenResults14 := util.CalculateMeld(playerInfo, targetTile34, isRedFive, allowChi)
	if len(results14) == 0 && len(incShantenResults14) == 0 {
		return nil // fmt.Errorf("Input error: cannot meld this tile")
	}

	// Meld
	humanTiles := humanHands(playerInfo)
	handsTobeNaki := humanTiles + " " + model.SepTargetTile + " " + util.Tile34ToStr(targetTile34) + "?"
	fmt.Println(handsTobeNaki)
	fmt.Println(strings.Repeat("=", len(handsTobeNaki)))

	// Original hand analysis result
	fmt.Println("Current " + util.NumberToChineseShanten(result.Shanten) + ":")
	r := &analysisResult{
		discardTile34:  -1,
		result13:       result,
		mixedRiskTable: mixedRiskTable,
	}
	r.printWaitsWithImproves13_oneRow()

	// Prompt information
	// TODO: When the round income is similar, prompt: similar round income, pursue win rate by discarding xx, pursue points by discarding xx
	if shanten == -1 {
		color.HiRed("【Winning Hand】")
	} else if shanten <= 1 {
		// After meld, if in tenpai or one shanten, prompt for shape tenpai
		if len(results14) > 0 && results14[0].LeftDrawTilesCount > 0 && results14[0].LeftDrawTilesCount <= 16 {
			color.HiGreen("Consider shape tenpai?")
		}
	}

	// TODO: Near the end of the round, prompt which player's discard is at the bottom of the river

	// Meld discard analysis result
	printResults14WithRisk(results14, mixedRiskTable)
	printResults14WithRisk(incShantenResults14, mixedRiskTable)
	return nil
}

func analysisHumanTiles(humanTilesInfo *model.HumanTilesInfo) (playerInfo *model.PlayerInfo, err error) {
	defer func() {
		if er := recover(); er != nil {
			err = er.(error)
		}
	}()

	if err = humanTilesInfo.SelfParse(); err != nil {
		return
	}

	tiles34, numRedFives, err := util.StrToTiles34(humanTilesInfo.HumanTiles)
	if err != nil {
		return
	}

	tileCount := util.CountOfTiles34(tiles34)
	if tileCount > 14 {
		return nil, fmt.Errorf("Input error: %d tiles", tileCount)
	}

	if tileCount%3 == 0 {
		color.HiYellow("%s is %d tiles\nA tile was randomly added by the assistant", humanTilesInfo.HumanTiles, tileCount)
		util.RandomAddTile(tiles34)
	}

	melds := []model.Meld{}
	for _, humanMeld := range humanTilesInfo.HumanMelds {
		tiles, _numRedFives, er := util.StrToTiles(humanMeld)
		if er != nil {
			return nil, er
		}
		isUpper := humanMeld[len(humanMeld)-1] <= 'Z'
		var meldType int
		switch {
		case len(tiles) == 3 && tiles[0] != tiles[1]:
			meldType = model.MeldTypeChi
		case len(tiles) == 3 && tiles[0] == tiles[1]:
			meldType = model.MeldTypePon
		case len(tiles) == 4 && isUpper:
			meldType = model.MeldTypeAnkan
		case len(tiles) == 4 && !isUpper:
			meldType = model.MeldTypeMinkan
		default:
			return nil, fmt.Errorf("Input error: %s", humanMeld)
		}
		containRedFive := false
		for i, c := range _numRedFives {
			if c > 0 {
				containRedFive = true
				numRedFives[i] += c
			}
		}
		melds = append(melds, model.Meld{
			MeldType:       meldType,
			Tiles:          tiles,
			ContainRedFive: containRedFive,
		})
	}

	playerInfo = model.NewSimplePlayerInfo(tiles34, melds)
	playerInfo.NumRedFives = numRedFives

	if humanTilesInfo.HumanDoraTiles != "" {
		playerInfo.DoraTiles, _, err = util.StrToTiles(humanTilesInfo.HumanDoraTiles)
		if err != nil {
			return
		}
	}

	if humanTilesInfo.HumanTargetTile != "" {
		if tileCount%3 == 2 {
			return nil, fmt.Errorf("Input error: %s is %d tiles", humanTilesInfo.HumanTiles, tileCount)
		}
		targetTile34, isRedFive, er := util.StrToTile34(humanTilesInfo.HumanTargetTile)
		if er != nil {
			return nil, er
		}
		if er := analysisMeld(playerInfo, targetTile34, isRedFive, true, nil); er != nil {
			return nil, er
		}
		return
	}

	playerInfo.IsTsumo = humanTilesInfo.IsTsumo
	err = analysisPlayerWithRisk(playerInfo, nil)
	return
}
