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
	if shanten == 1 && len(playerInfo.DiscardTiles) < 9 && len(results14) > 0 && len(incShantenResults14) > 0 && !playerInfo.IsNaki() { // 鳴きの際の向聴戻りは一時的に考慮しない
		if results14[0].Result13.Waits.AllCount() < 9 && results14[0].Result13.MixedWaitsScore < incShantenResults14[0].Result13.MixedWaitsScore {
			bestAttackDiscardTile = incShantenResults14[0].DiscardTile
		}
	}
	return bestAttackDiscardTile
}

// TODO: model へリファクタリング
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
	// 手牌
	humanTiles := humanHands(playerInfo)
	fmt.Println(humanTiles)
	fmt.Println(strings.Repeat("=", len(humanTiles)))

	countOfTiles := util.CountOfTiles34(playerInfo.HandTiles34)
	switch countOfTiles % 3 {
	case 1:
		result := util.CalculateShantenWithImproves13(playerInfo)
		fmt.Println("現在" + util.NumberToChineseShanten(result.Shanten) + "：")
		r := &analysisResult{
			discardTile34:  -1,
			result13:       result,
			mixedRiskTable: mixedRiskTable,
		}
		r.printWaitsWithImproves13_oneRow()
	case 2:
		// 手牌分析
		shanten, results14, incShantenResults14 := util.CalculateShantenWithImproves14(playerInfo)

		// 提案情報
		if shanten == -1 {
			color.HiRed("【和了】")
		} else if shanten == 0 {
			if len(results14) > 0 {
				r13 := results14[0].Result13
				if r13.RiichiPoint > 0 && r13.FuritenRate == 0 && r13.DamaPoint >= 5200 && r13.DamaWaits.AllCount() == r13.Waits.AllCount() {
					color.HiGreen("点数が十分なダマテン：ダマテンを狙う、リーチで点数を狙う")
				}
				// 局収支が近い場合、提案：局収支が近い、和了率を追求するならxx、点数を追求するならxx
			}
		} else if shanten == 1 {
			// 序盤・中盤の門前で、向聴戻りを注意
			if len(playerInfo.DiscardTiles) < 9 && !playerInfo.IsNaki() {
				alertBackwardToShanten2(results14, incShantenResults14)
			}
		}

		// TODO: 流局に近い場合、河底が誰かを提示

		// 何切る分析結果
		printResults14WithRisk(results14, mixedRiskTable)
		printResults14WithRisk(incShantenResults14, mixedRiskTable)
	default:
		err := fmt.Errorf("パラメータエラー: %d 枚の牌", countOfTiles)
		if debugMode {
			panic(err)
		}
		return err
	}

	fmt.Println()
	return nil
}

// 分析鸣牌
// playerInfo: 自家信息
// targetTile34: 他家舍牌
// isRedFive: 此舍牌是否为赤5
// allowChi: 是否能吃
// mixedRiskTable: 危险度表
func analysisMeld(playerInfo *model.PlayerInfo, targetTile34 int, isRedFive bool, allowChi bool, mixedRiskTable riskTable) error {
	if handsCount := util.CountOfTiles34(playerInfo.HandTiles34); handsCount%3 != 1 {
		return fmt.Errorf("手牌错误：%d 张牌 %v", handsCount, playerInfo.HandTiles34)
	}
	// 原始手牌分析
	result := util.CalculateShantenWithImproves13(playerInfo)
	// 副露分析
	shanten, results14, incShantenResults14 := util.CalculateMeld(playerInfo, targetTile34, isRedFive, allowChi)
	if len(results14) == 0 && len(incShantenResults14) == 0 {
		return nil // fmt.Errorf("输入错误：无法鸣这张牌")
	}

	// 鸣牌
	humanTiles := humanHands(playerInfo)
	handsTobeNaki := humanTiles + " " + model.SepTargetTile + " " + util.Tile34ToStr(targetTile34) + "?"
	fmt.Println(handsTobeNaki)
	fmt.Println(strings.Repeat("=", len(handsTobeNaki)))

	// 原始手牌分析结果
	fmt.Println("当前" + util.NumberToChineseShanten(result.Shanten) + "：")
	r := &analysisResult{
		discardTile34:  -1,
		result13:       result,
		mixedRiskTable: mixedRiskTable,
	}
	r.printWaitsWithImproves13_oneRow()

	// 提示信息
	// TODO: 局收支相近时，提示：局收支相近，追求和率打xx，追求打点打xx
	if shanten == -1 {
		color.HiRed("【已和牌】")
	} else if shanten <= 1 {
		// 鸣牌后听牌或一向听，提示型听
		if len(results14) > 0 && results14[0].LeftDrawTilesCount > 0 && results14[0].LeftDrawTilesCount <= 16 {
			color.HiGreen("考虑型听？")
		}
	}

	// TODO: 接近流局时提示河底是哪家

	// 鸣牌何切分析结果
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
		return nil, fmt.Errorf("输入错误：%d 张牌", tileCount)
	}

	if tileCount%3 == 0 {
		color.HiYellow("%s は %d 枚の牌です\nアシスタントがランダムで1枚追加しました", humanTilesInfo.HumanTiles, tileCount)
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
			return nil, fmt.Errorf("输入错误: %s", humanMeld)
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
			return nil, fmt.Errorf("输入错误: %s 是 %d 张牌", humanTilesInfo.HumanTiles, tileCount)
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
