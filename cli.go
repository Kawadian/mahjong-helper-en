package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/EndlessCheng/mahjong-helper/util"
	"github.com/fatih/color"
)

func printAccountInfo(accountID int) {
	fmt.Printf("Your account ID is ")
	color.New(color.FgHiGreen).Printf("%d", accountID)
	fmt.Printf(", this number is the ID in the Mahjong Soul server account database, the smaller the value, the earlier your registration time\n")
}

//

func (p *playerInfo) printDiscards() {
	// TODO: Highlight unreasonable or dangerous discards, such as
	// - Discarding middle tiles at the beginning
	// - After starting to discard middle tiles, discarding terminal tiles (it could also be because someone called a tile, e.g., 133m someone called 2m)
	// - Discarding dora, give a reminder
	// - Discarding red dora
	// - When someone declares riichi, repeatedly discarding dangerous tiles (it could be that the opponent has read the tiles correctly, or the tiles in the opponent's hand and the river tiles combined create safe tiles)
	// - Refer to the translation of "Demon God's Eye" on Tieba for more details https://tieba.baidu.com/p/3311909701
	//      For example, if a pair is discarded early, it is unlikely to be a seven pairs hand.
	//      If the opponent discards a two-sided wait early, it can be inferred that they are going for a flush or a pair-based hand. If they declare riichi or call a tile, it is easier to read their hand.
	// https://tieba.baidu.com/p/3311909701
	//      Remember the tiles discarded after calling and in the late game, discard the safe tiles before the opponent discards them
	// https://tieba.baidu.com/p/3372239806
	//      The tiles discarded when calling are dangerous; all tiles discarded after calling are dangerous

	fmt.Printf(p.name + ":")
	for i, disTile := range p.discardTiles {
		fmt.Printf(" ")
		// TODO: Display dora, red dora
		bgColor := color.BgBlack
		fgColor := color.FgWhite
		var tile string
		if disTile >= 0 { // Hand discard
			tile = util.Mahjong[disTile]
			if disTile >= 27 {
				tile = util.MahjongU[disTile] // Pay attention to hand discards of honor tiles
			}
			if p.isNaki { // Open meld
				fgColor = getOtherDiscardAlertColor(disTile) // Highlight middle tile hand discards
				if util.InInts(i, p.meldDiscardsAt) {
					bgColor = color.BgWhite // Highlight the background of the tile discarded when calling
					fgColor = color.FgBlack
				}
			}
		} else { // Draw discard
			disTile = ^disTile
			tile = util.Mahjong[disTile]
			fgColor = color.FgHiBlack // Display in dark color
		}
		color.New(bgColor, fgColor).Print(tile)
	}
	fmt.Println()
}

//

type handsRisk struct {
	tile int
	risk float64
}

// Risk level of 34 types of tiles
type riskTable util.RiskTiles34

func (t riskTable) printWithHands(hands []int, fixedRiskMulti float64) (containLine bool) {
	// Print tiles with a risk of 0 (safe tiles, or NC with remaining count = 0)
	safeCount := 0
	for i, c := range hands {
		if c > 0 && t[i] == 0 {
			fmt.Printf(" " + util.MahjongZH[i])
			safeCount++
		}
	}

	// Print dangerous tiles, sorted by risk & highlighted
	handsRisks := []handsRisk{}
	for i, c := range hands {
		if c > 0 && t[i] > 0 {
			handsRisks = append(handsRisks, handsRisk{i, t[i]})
		}
	}
	sort.Slice(handsRisks, func(i, j int) bool {
		return handsRisks[i].risk < handsRisks[j].risk
	})
	if len(handsRisks) > 0 {
		if safeCount > 0 {
			fmt.Print(" |")
			containLine = true
		}
		for _, hr := range handsRisks {
			// Color considers the tenpai rate
			color.New(getNumRiskColor(hr.risk * fixedRiskMulti)).Printf(" " + util.MahjongZH[hr.tile])
		}
	}

	return
}

func (t riskTable) getBestDefenceTile(tiles34 []int) (result int) {
	minRisk := 100.0
	maxRisk := 0.0
	for tile, c := range tiles34 {
		if c == 0 {
			continue
		}
		risk := t[tile]
		if risk < minRisk {
			minRisk = risk
			result = tile
		}
		if risk > maxRisk {
			maxRisk = risk
		}
	}
	if maxRisk == 0 {
		return -1
	}
	return result
}

//

type riskInfo struct {
	// 3 players for three-player mahjong, 4 players for four-player mahjong
	playerNumber int

	// Tenpai rate of the player (100.0 when declaring riichi)
	tenpaiRate float64

	// Safe tiles of the player
	// If the player has a kan operation, the kan tile is also considered a safe tile, which helps in determining the danger level of suji tiles
	safeTiles34 []bool

	// Risk table of various tiles
	riskTable riskTable

	// Remaining no suji 123789
	// A total of 18 types. The fewer remaining no suji tiles, the more dangerous the no suji tile
	leftNoSujiTiles []int

	// Whether it is a tsumogiri riichi
	isTsumogiriRiichi bool

	// Ron points
	// For debugging purposes only
	_ronPoint float64
}

type riskInfoList []*riskInfo

// Comprehensive risk level considering the tenpai rate
func (l riskInfoList) mixedRiskTable() riskTable {
	mixedRiskTable := make(riskTable, 34)
	for i := range mixedRiskTable {
		mixedRisk := 0.0
		for _, ri := range l[1:] {
			if ri.tenpaiRate <= 15 {
				continue
			}
			_risk := ri.riskTable[i] * ri.tenpaiRate / 100
			mixedRisk = mixedRisk + _risk - mixedRisk*_risk/100
		}
		mixedRiskTable[i] = mixedRisk
	}
	return mixedRiskTable
}

func (l riskInfoList) printWithHands(hands []int, leftCounts []int) {
	// Print the risk level if the tenpai rate exceeds a certain value
	const (
		minShownTenpaiRate4 = 50.0
		minShownTenpaiRate3 = 20.0
	)

	minShownTenpaiRate := minShownTenpaiRate4
	if l[0].playerNumber == 3 {
		minShownTenpaiRate = minShownTenpaiRate3
	}

	dangerousPlayerCount := 0
	// Print safe tiles and dangerous tiles
	names := []string{"", "Lower player", "Opposite player", "Upper player"}
	for i := len(l) - 1; i >= 1; i-- {
		tenpaiRate := l[i].tenpaiRate
		if len(l[i].riskTable) > 0 && (debugMode || tenpaiRate > minShownTenpaiRate) {
			dangerousPlayerCount++
			fmt.Print(names[i] + " safe tiles:")
			//if debugMode {
			//fmt.Printf("(%d*%2.2f%% tenpai rate)", int(l[i]._ronPoint), l[i].tenpaiRate)
			//}
			containLine := l[i].riskTable.printWithHands(hands, tenpaiRate/100)

			// Print tenpai rate
			fmt.Print(" ")
			if !containLine {
				fmt.Print("  ")
			}
			fmt.Print("[")
			if tenpaiRate == 100 {
				fmt.Print("100.%")
			} else {
				fmt.Printf("%4.1f%%", tenpaiRate)
			}
			fmt.Print(" tenpai rate]")

			// Print the number of no suji tiles
			fmt.Print(" ")
			const badMachiLimit = 3
			noSujiInfo := ""
			if l[i].isTsumogiriRiichi {
				noSujiInfo = "Tsumogiri riichi"
			} else if len(l[i].leftNoSujiTiles) == 0 {
				noSujiInfo = "Bad wait/Chombo"
			} else if len(l[i].leftNoSujiTiles) <= badMachiLimit {
				noSujiInfo = "Possible bad wait/Chombo"
			}
			if noSujiInfo != "" {
				fmt.Printf("[%d no suji: ", len(l[i].leftNoSujiTiles))
				color.New(color.FgHiYellow).Printf("%s", noSujiInfo)
				fmt.Print("]")
			} else {
				fmt.Printf("[%d no suji]", len(l[i].leftNoSujiTiles))
			}

			fmt.Println()
		}
	}

	// If more than one player declares riichi or calls a tile, print the weighted comprehensive risk level (considering the tenpai rate)
	mixedPlayers := 0
	for _, ri := range l[1:] {
		if ri.tenpaiRate > 0 {
			mixedPlayers++
		}
	}
	if dangerousPlayerCount > 0 && mixedPlayers > 1 {
		fmt.Print("Comprehensive safe tiles:")
		mixedRiskTable := l.mixedRiskTable()
		mixedRiskTable.printWithHands(hands, 1)
		fmt.Println()
	}

	// Print safe tiles generated by NC and OC
	// TODO: Refactor to another function
	if dangerousPlayerCount > 0 {
		ncSafeTileList := util.CalcNCSafeTiles(leftCounts).FilterWithHands(hands)
		ocSafeTileList := util.CalcOCSafeTiles(leftCounts).FilterWithHands(hands)
		if len(ncSafeTileList) > 0 {
			fmt.Printf("NC:")
			for _, safeTile := range ncSafeTileList {
				fmt.Printf(" " + util.MahjongZH[safeTile.Tile34])
			}
			fmt.Println()
		}
		if len(ocSafeTileList) > 0 {
			fmt.Printf("OC:")
			for _, safeTile := range ocSafeTileList {
				fmt.Printf(" " + util.MahjongZH[safeTile.Tile34])
			}
			fmt.Println()
		}

		// The following is another display method: displaying wall tiles
		//printedNC := false
		//for i, c := range leftCounts[:27] {
		//	if c != 0 || i%9 == 0 || i%9 == 8 {
		//		continue
		//	}
		//	if !printedNC {
		//		printedNC = true
		//		fmt.Printf("NC:")
		//	}
		//	fmt.Printf(" " + util.MahjongZH[i])
		//}
		//if printedNC {
		//	fmt.Println()
		//}
		//printedOC := false
		//for i, c := range leftCounts[:27] {
		//	if c != 1 || i%9 == 0 || i%9 == 8 {
		//		continue
		//	}
		//	if !printedOC {
		//		printedOC = true
		//		fmt.Printf("OC:")
		//	}
		//	fmt.Printf(" " + util.MahjongZH[i])
		//}
		//if printedOC {
		//	fmt.Println()
		//}
		fmt.Println()
	}
}

//

func alertBackwardToShanten2(results util.Hand14AnalysisResultList, incShantenResults util.Hand14AnalysisResultList) {
	if len(results) == 0 || len(incShantenResults) == 0 {
		return
	}

	if results[0].Result13.Waits.AllCount() < 9 {
		if results[0].Result13.MixedWaitsScore < incShantenResults[0].Result13.MixedWaitsScore {
			color.HiGreen("Backward to shanten?")
		}
	}
}

// Yaku types to alert
var yakuTypesToAlert = []int{
	//util.YakuKokushi,
	//util.YakuKokushi13,
	util.YakuSuuAnkou,
	util.YakuSuuAnkouTanki,
	util.YakuDaisangen,
	util.YakuShousuushii,
	util.YakuDaisuushii,
	util.YakuTsuuiisou,
	util.YakuChinroutou,
	util.YakuRyuuiisou,
	util.YakuChuuren,
	util.YakuChuuren9,
	util.YakuSuuKantsu,
	//util.YakuTenhou,
	//util.YakuChiihou,

	util.YakuChiitoi,
	util.YakuPinfu,
	util.YakuRyanpeikou,
	util.YakuIipeikou,
	util.YakuSanshokuDoujun,
	util.YakuIttsuu,
	util.YakuToitoi,
	util.YakuSanAnkou,
	util.YakuSanshokuDoukou,
	util.YakuSanKantsu,
	util.YakuTanyao,
	util.YakuChanta,
	util.YakuJunchan,
	util.YakuHonroutou,
	util.YakuShousangen,
	util.YakuHonitsu,
	util.YakuChinitsu,

	util.YakuShiiaruraotai,
	util.YakuUumensai,
	util.YakuSanrenkou,
	util.YakuIsshokusanjun,
}

/*

8     Discard 3s Wait [2m, 7m]
9.20  [20 Improvement]  4.00 Wait count

4     Wait [2m, 7m]
4.50  [ 4 Improvement]  55.36% Reference win rate

8     Call 45m, discard 4m Wait [2m, 7m]
9.20  [20 Improvement]  4.00 Wait count

*/
// Print hand analysis results (two rows)
func printWaitsWithImproves13_twoRows(result13 *util.Hand13AnalysisResult, discardTile34 int, openTiles34 []int) {
	shanten := result13.Shanten
	waits := result13.Waits

	waitsCount, waitTiles := waits.ParseIndex()
	c := getWaitsCountColor(shanten, float64(waitsCount))
	color.New(c).Printf("%-6d", waitsCount)
	if discardTile34 != -1 {
		if len(openTiles34) > 0 {
			meldType := "Call"
			if openTiles34[0] == openTiles34[1] {
				meldType = "Pong"
			}
			color.New(color.FgHiWhite).Printf("%s%s", string([]rune(util.MahjongZH[openTiles34[0]])[:1]), util.MahjongZH[openTiles34[1]])
			fmt.Printf("%s, ", meldType)
		}
		fmt.Print("Discard ")
		fmt.Print(util.MahjongZH[discardTile34])
		fmt.Print(" ")
	}
	//fmt.Print("Wait")
	//if shanten <= 1 {
	//	fmt.Print("[")
	//	if len(waitTiles) > 0 {
	//		fmt.Print(util.MahjongZH[waitTiles[0]])
	//		for _, idx := range waitTiles[1:] {
	//			fmt.Print(", " + util.MahjongZH[idx])
	//		}
	//	}
	//	fmt.Println("]")
	//} else {
	fmt.Println(util.TilesToStrWithBracket(waitTiles))
	//}

	if len(result13.Improves) > 0 {
		fmt.Printf("%-6.2f[%2d Improvement]", result13.AvgImproveWaitsCount, len(result13.Improves))
	} else {
		fmt.Print(strings.Repeat(" ", 15))
	}

	fmt.Print(" ")

	if shanten >= 1 {
		c := getWaitsCountColor(shanten-1, result13.AvgNextShantenWaitsCount)
		color.New(c).Printf("%5.2f", result13.AvgNextShantenWaitsCount)
		fmt.Printf(" %s", util.NumberToChineseShanten(shanten-1))
		if shanten >= 2 {
			fmt.Printf("Waits")
		} else { // shanten == 1
			fmt.Printf("Count")
			if showAgariAboveShanten1 {
				fmt.Printf("（%.2f%% Reference win rate）", result13.AvgAgariRate)
			}
		}
		if showScore {
			mixedScore := result13.MixedWaitsScore
			//for i := 2; i <= shanten; i++ {
			//	mixedScore /= 4
			//}
			fmt.Printf("（%.2f Comprehensive score）", mixedScore)
		}
	} else { // shanten == 0
		fmt.Printf("%5.2f%% Reference win rate", result13.AvgAgariRate)
	}

	fmt.Println()
}

type analysisResult struct {
	discardTile34     int
	isDiscardTileDora bool
	openTiles34       []int
	result13          *util.Hand13AnalysisResult

	mixedRiskTable riskTable

	highlightAvgImproveWaitsCount bool
	highlightMixedScore           bool
}

/*
4[ 4.56] Discard 8p => 44.50% Reference win rate [ 4 Improvement] [7p 7s] [Silent 2000] [Sanshoku] [Chombo]

4[ 4.56] Discard 8p => 0.00% Reference win rate [ 4 Improvement] [7p 7s] [No yaku]

31[33.58] Discard 7s =>  5.23 Wait count [19.21 Speed] [16 Improvement] [6789p 56789s] [Round income 3120] [Possible chombo]

48[50.64] Discard 5p => 24.25 One shanten [12 Improvement] [123456789p 56789s]

31[33.62] Call 77s, discard 5p => 5.


*/
// Print hand analysis results (single row)
func (r *analysisResult) printWaitsWithImproves13_oneRow() {
	discardTile34 := r.discardTile34
	openTiles34 := r.openTiles34
	result13 := r.result13

	shanten := result13.Shanten

	// Number of waits
	waitsCount := result13.Waits.AllCount()
	c := getWaitsCountColor(shanten, float64(waitsCount))
	color.New(c).Printf("%2d", waitsCount)
	// Average number of improvements
	if len(result13.Improves) > 0 {
		if r.highlightAvgImproveWaitsCount {
			color.New(color.FgHiWhite).Printf("[%5.2f]", result13.AvgImproveWaitsCount)
		} else {
			fmt.Printf("[%5.2f]", result13.AvgImproveWaitsCount)
		}
	} else {
		fmt.Print(strings.Repeat(" ", 7))
	}

	fmt.Print(" ")

	// Whether it is a 3k+2 tile analysis
	if discardTile34 != -1 {
		// Meld analysis
		if len(openTiles34) > 0 {
			meldType := "Chii"
			if openTiles34[0] == openTiles34[1] {
				meldType = "Pong"
			}
			color.New(color.FgHiWhite).Printf("%s%s", string([]rune(util.MahjongZH[openTiles34[0]])[:1]), util.MahjongZH[openTiles34[1]])
			fmt.Printf("%s,", meldType)
		}
		// Discard tile
		if r.isDiscardTileDora {
			color.New(color.FgHiWhite).Print("Dora")
		} else {
			fmt.Print("Discard")
		}
		tileZH := util.MahjongZH[discardTile34]
		if discardTile34 >= 27 {
			tileZH = " " + tileZH
		}
		if r.mixedRiskTable != nil {
			// If there is an actual risk, display the discard risk based on the actual risk
			risk := r.mixedRiskTable[discardTile34]
			if risk == 0 {
				fmt.Print(tileZH)
			} else {
				color.New(getNumRiskColor(risk)).Print(tileZH)
			}
		} else {
			fmt.Print(tileZH)
		}
	}

	fmt.Print(" => ")

	if shanten >= 1 {
		// Average number of waits after advancing
		incShanten := shanten - 1
		c := getWaitsCountColor(incShanten, result13.AvgNextShantenWaitsCount)
		color.New(c).Printf("%5.2f", result13.AvgNextShantenWaitsCount)
		fmt.Printf("%s", util.NumberToChineseShanten(incShanten))
		if incShanten >= 1 {
			//fmt.Printf("Waits")
		} else { // incShanten == 0
			fmt.Printf("Count")
			//if showAgariAboveShanten1 {
			//	fmt.Printf("（%.2f%% Reference win rate）", result13.AvgAgariRate)
			//}
		}
	} else { // shanten == 0
		// Reference win rate after advancing
		// If furiten or partial wait, mark in red
		if result13.FuritenRate == 1 || result13.IsPartWait {
			color.New(color.FgHiRed).Printf("%5.2f%% Reference win rate", result13.AvgAgariRate)
		} else {
			fmt.Printf("%5.2f%% Reference win rate", result13.AvgAgariRate)
		}
	}

	// Hand speed, used for quick rounds
	if result13.MixedWaitsScore > 0 && shanten >= 1 && shanten <= 2 {
		fmt.Print(" ")
		if r.highlightMixedScore {
			color.New(color.FgHiWhite).Printf("[%5.2f Speed]", result13.MixedWaitsScore)
		} else {
			fmt.Printf("[%5.2f Speed]", result13.MixedWaitsScore)
		}
	}

	// Round income
	if showScore && result13.MixedRoundPoint != 0.0 {
		fmt.Print(" ")
		color.New(color.FgHiGreen).Printf("[Round income %4d]", int(math.Round(result13.MixedRoundPoint)))
	}

	// (Silent) Ron points
	if result13.DamaPoint > 0 {
		fmt.Print(" ")
		ronType := "Ron"
		if !result13.IsNaki {
			ronType = "Silent"
		}
		color.New(color.FgHiGreen).Printf("[%s %d]", ronType, int(math.Round(result13.DamaPoint)))
	}

	// Riichi points, considering tsumo, ippatsu, ura dora
	if result13.RiichiPoint > 0 {
		fmt.Print(" ")
		color.New(color.FgHiGreen).Printf("[Riichi %d]", int(math.Round(result13.RiichiPoint)))
	}

	if len(result13.YakuTypes) > 0 {
		// Yaku types (displayed within two shanten)
		if result13.Shanten <= 2 {
			if !showAllYakuTypes && !debugMode {
				shownYakuTypes := []int{}
				for yakuType := range result13.YakuTypes {
					for _, yt := range yakuTypesToAlert {
						if yakuType == yt {
							shownYakuTypes = append(shownYakuTypes, yakuType)
						}
					}
				}
				if len(shownYakuTypes) > 0 {
					sort.Ints(shownYakuTypes)
					fmt.Print(" ")
					color.New(color.FgHiGreen).Printf(util.YakuTypesToStr(shownYakuTypes))
				}
			} else {
				// debug
				fmt.Print(" ")
				color.New(color.FgHiGreen).Printf(util.YakuTypesWithDoraToStr(result13.YakuTypes, result13.DoraCount))
			}
			// Partial wait
			if result13.IsPartWait {
				fmt.Print(" ")
				color.New(color.FgHiRed).Printf("[Partial wait]")
			}
		}
	} else if result13.IsNaki && shanten >= 0 && shanten <= 2 {
		// No yaku alert when calling (from tenpai to two shanten)
		fmt.Print(" ")
		color.New(color.FgHiRed).Printf("[No yaku]")
	}

	// Furiten alert
	if result13.FuritenRate > 0 {
		fmt.Print(" ")
		if result13.FuritenRate < 1 {
			color.New(color.FgHiYellow).Printf("[Possible furiten]")
		} else {
			color.New(color.FgHiRed).Printf("[Furiten]")
		}
	}

	// Number of improvements
	if showScore {
		fmt.Print(" ")
		if len(result13.Improves) > 0 {
			fmt.Printf("[%2d Improvements]", len(result13.Improves))
		} else {
			fmt.Print(strings.Repeat(" ", 4))
			fmt.Print(strings.Repeat("　", 2)) // Full-width space
		}
	}

	// Types of waits
	fmt.Print(" ")
	waitTiles := result13.Waits.AvailableTiles()
	fmt.Print(util.TilesToStrWithBracket(waitTiles))

	//

	fmt.Println()

	if showImproveDetail {
		for tile, waits := range result13.Improves {
			fmt.Printf("Draw %s improves to %s\n", util.Mahjong[tile], waits.String())
		}
	}
}

func printResults14WithRisk(results14 util.Hand14AnalysisResultList, mixedRiskTable riskTable) {
	if len(results14) == 0 {
		return
	}

	maxMixedScore := -1.0
	maxAvgImproveWaitsCount := -1.0
	for _, result := range results14 {
		if result.Result13.MixedWaitsScore > maxMixedScore {
			maxMixedScore = result.Result13.MixedWaitsScore
		}
		if result.Result13.AvgImproveWaitsCount > maxAvgImproveWaitsCount {
			maxAvgImproveWaitsCount = result.Result13.AvgImproveWaitsCount
		}
	}

	if len(results14[0].OpenTiles) > 0 {
		fmt.Print("After calling")
	}
	fmt.Println(util.NumberToChineseShanten(results14[0].Result13.Shanten) + ":")

	if results14[0].Result13.Shanten == 0 {
		// Check if the waits are the same but the points are different
		isDiffPoint := false
		baseWaits := results14[0].Result13.Waits
		baseDamaPoint := results14[0].Result13.DamaPoint
		baseRiichiPoint := results14[0].Result13.RiichiPoint
		for _, result14 := range results14[1:] {
			if baseWaits.Equals(result14.Result13.Waits) && (baseDamaPoint != result14.Result13.DamaPoint || baseRiichiPoint != result14.Result13.RiichiPoint) {
				isDiffPoint = true
				break
			}
		}

		if isDiffPoint {
			color.HiGreen("Pay attention to discard choice: points")
		}
	}

	// FIXME: How to simplify the discard options when there are many choices?
	//const maxShown = 10
	//if len(results14) > maxShown { // Limit the number of outputs
	//	results14 = results14[:maxShown]
	//}
	for _, result := range results14 {
		r := &analysisResult{
			result.DiscardTile,
			result.IsDiscardDoraTile,
			result.OpenTiles,
			result.Result13,
			mixedRiskTable,
			result.Result13.AvgImproveWaitsCount == maxAvgImproveWaitsCount,
			result.Result13.MixedWaitsScore == maxMixedScore,
		}
		r.printWaitsWithImproves13_oneRow()
	}
}
