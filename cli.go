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
	fmt.Printf("あなたのアカウントIDは ")
	color.New(color.FgHiGreen).Printf("%d", accountID)
	fmt.Printf(" です。この数字は雀魂サーバーのアカウントデータベースのIDで、この値が小さいほど登録時間が早いことを示します。\n")
}

//

func (p *playerInfo) printDiscards() {
	// TODO: 不合理な捨て牌や危険な捨て牌をハイライトする
	// - 最初から中張牌を切る
	// - 中張牌を切った後、幺九牌を手切りする（例えば133mで2mをポンされた場合）
	// - ドラを切る、警告する
	// - 赤宝牌を切る
	// - 誰かがリーチしている場合、何度も危険度の高い牌を切る（相手が牌を読んでいる可能性がある、または相手の手牌と牌河が合わさって安牌ができる）
	// - その他は掲示板の「魔神の目」の翻訳を参考にする https://tieba.baidu.com/p/3311909701
	//      例えば、手切りで対子を切った場合、基本的に七対子ではないことがわかります。
	//      もし相手が早巡で両面搭子を手切りした場合、染め手を狙っているか、対子型の手牌であることが推測できます。リーチや鳴きが入った場合も、手牌を読みやすくなります。
	// https://tieba.baidu.com/p/3311909701
	//      鳴きの後や終盤の手切り牌を覚えておくべきで、他人の手切り前の安牌を先に切るべきです。
	// https://tieba.baidu.com/p/3372239806
	//      鳴きの際に打ち出された牌の色は危険です。ポンの後はすべての牌が危険です。

	fmt.Printf(p.name + ":")
	for i, disTile := range p.discardTiles {
		fmt.Printf(" ")
		// TODO: ドラ、赤宝牌を表示する
		bgColor := color.BgBlack
		fgColor := color.FgWhite
		var tile string
		if disTile >= 0 { // 手切り
			tile = util.Mahjong[disTile]
			if disTile >= 27 {
				tile = util.MahjongU[disTile] // 字牌の手切りに注目
			}
			if p.isNaki { // 副露
				fgColor = getOtherDiscardAlertColor(disTile) // 中張牌の手切りをハイライト
				if util.InInts(i, p.meldDiscardsAt) {
					bgColor = color.BgWhite // 鳴きの際に切った牌は背景をハイライト
					fgColor = color.FgBlack
				}
			}
		} else { // 摸切り
			disTile = ^disTile
			tile = util.Mahjong[disTile]
			fgColor = color.FgHiBlack // 暗色表示
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

// 34種類の牌の危険度
type riskTable util.RiskTiles34

func (t riskTable) printWithHands(hands []int, fixedRiskMulti float64) (containLine bool) {
	// 鋳率=0の牌を表示する（現物、またはNCで残り数=0）
	safeCount := 0
	for i, c := range hands {
		if c > 0 && t[i] == 0 {
			fmt.Printf(" " + util.MahjongZH[i])
			safeCount++
		}
	}

	// 危険牌を表示し、鋳率でソート＆ハイライト
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
			// 色は聴牌率を考慮
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
	// 三麻は3、四麻は4
	playerNumber int

	// このプレイヤーの聴牌率（リーチ時は100.0）
	tenpaiRate float64

	// このプレイヤーの安牌
	// このプレイヤーが槓操作を行った場合、その槓の牌も安牌と見なす。これは筋壁の危険度を判断するのに役立つ。
	safeTiles34 []bool

	// 各種牌の鋳率表
	riskTable riskTable

	// 残り無筋123789
	// 合計18種類。残り無筋牌の数が少ないほど、その無筋牌は危険。
	leftNoSujiTiles []int

	// 摸切りリーチかどうか
	isTsumogiriRiichi bool

	// 栄和点数
	// デバッグ用
	_ronPoint float64
}

type riskInfoList []*riskInfo

// 聴牌率を考慮した総合危険度
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
	// 聴牌率が一定値を超えたら鋳率を表示
	const (
		minShownTenpaiRate4 = 50.0
		minShownTenpaiRate3 = 20.0
	)

	minShownTenpaiRate := minShownTenpaiRate4
	if l[0].playerNumber == 3 {
		minShownTenpaiRate = minShownTenpaiRate3
	}

	dangerousPlayerCount := 0
	// 安牌、危険牌を表示
	names := []string{"", "下家", "対面", "上家"}
	for i := len(l) - 1; i >= 1; i-- {
		tenpaiRate := l[i].tenpaiRate
		if len(l[i].riskTable) > 0 && (debugMode || tenpaiRate > minShownTenpaiRate) {
			dangerousPlayerCount++
			fmt.Print(names[i] + "安牌:")
			//if debugMode {
			//fmt.Printf("(%d*%2.2f%%聴牌率)", int(l[i]._ronPoint), l[i].tenpaiRate)
			//}
			containLine := l[i].riskTable.printWithHands(hands, tenpaiRate/100)

			// 聴牌率を表示
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
			fmt.Print("聴牌率]")

			// 無筋の数を表示
			fmt.Print(" ")
			const badMachiLimit = 3
			noSujiInfo := ""
			if l[i].isTsumogiriRiichi {
				noSujiInfo = "摸切りリーチ"
			} else if len(l[i].leftNoSujiTiles) == 0 {
				noSujiInfo = "愚形聴牌/振聴"
			} else if len(l[i].leftNoSujiTiles) <= badMachiLimit {
				noSujiInfo = "可能性のある愚形聴牌/振聴"
			}
			if noSujiInfo != "" {
				fmt.Printf("[%d無筋: ", len(l[i].leftNoSujiTiles))
				color.New(color.FgHiYellow).Printf("%s", noSujiInfo)
				fmt.Print("]")
			} else {
				fmt.Printf("[%d無筋]", len(l[i].leftNoSujiTiles))
			}

			fmt.Println()
		}
	}

	// 複数のプレイヤーがリーチ/副露している場合、加重総合鋳率を表示（聴牌率を考慮）
	mixedPlayers := 0
	for _, ri := range l[1:] {
		if ri.tenpaiRate > 0 {
			mixedPlayers++
		}
	}
	if dangerousPlayerCount > 0 && mixedPlayers > 1 {
		fmt.Print("総合安牌:")
		mixedRiskTable := l.mixedRiskTable()
		mixedRiskTable.printWithHands(hands, 1)
		fmt.Println()
	}

	// NC OCによる安牌を表示
	// TODO: 他の関数にリファクタリング
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

		// 以下は別の表示方法：壁牌を表示
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
			color.HiGreen("向聴戻り?")
		}
	}
}

// 注意が必要な役種
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

8     切 3索 待ち[2万, 7万]
9.20  [20 改良]  4.00 聴牌数

4     待ち [2万, 7万]
4.50  [ 4 改良]  55.36% 参考和了率

8     45万チー,切 4万 待ち[2万, 7万]
9.20  [20 改良]  4.00 聴牌数

*/
// 何切分析結果を表示（2行）
func printWaitsWithImproves13_twoRows(result13 *util.Hand13AnalysisResult, discardTile34 int, openTiles34 []int) {
	shanten := result13.Shanten
	waits := result13.Waits

	waitsCount, waitTiles := waits.ParseIndex()
	c := getWaitsCountColor(shanten, float64(waitsCount))
	color.New(c).Printf("%-6d", waitsCount)
	if discardTile34 != -1 {
		if len(openTiles34) > 0 {
			meldType := "チー"
			if openTiles34[0] == openTiles34[1] {
				meldType = "ポン"
			}
			color.New(color.FgHiWhite).Printf("%s%s", string([]rune(util.MahjongZH[openTiles34[0]])[:1]), util.MahjongZH[openTiles34[1]])
			fmt.Printf("%s,", meldType)
		}
		fmt.Print("切 ")
		fmt.Print(util.MahjongZH[discardTile34])
		fmt.Print(" ")
	}

	fmt.Println(util.TilesToStrWithBracket(waitTiles))

	if len(result13.Improves) > 0 {
		fmt.Printf("%-6.2f[%2d 改良]", result13.AvgImproveWaitsCount, len(result13.Improves))
	} else {
		fmt.Print(strings.Repeat(" ", 15))
	}

	fmt.Print(" ")

	if shanten >= 1 {
		c := getWaitsCountColor(shanten-1, result13.AvgNextShantenWaitsCount)
		color.New(c).Printf("%5.2f", result13.AvgNextShantenWaitsCount)
		fmt.Printf(" %s", util.NumberToChineseShanten(shanten-1))
		if shanten >= 2 {
			fmt.Printf("進張")
		} else { // shanten == 1
			fmt.Printf("数")
			if showAgariAboveShanten1 {
				fmt.Printf("（%.2f%% 参考和了率）", result13.AvgAgariRate)
			}
		}
		if showScore {
			mixedScore := result13.MixedWaitsScore
			fmt.Printf("（%.2f 総合点）", mixedScore)
		}
	} else { // shanten == 0
		fmt.Printf("%5.2f%% 参考和了率", result13.AvgAgariRate)
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
4[ 4.56] 切 8饼 => 44.50% 参考和率[ 4 改良] [7p 7s] [默听2000] [三色] [振听]

4[ 4.56] 切 8饼 => 0.00% 参考和率[ 4 改良] [7p 7s] [无役]

31[33.58] 切7索 =>  5.23听牌数 [19.21速度] [16改良] [6789p 56789s] [局收支3120] [可能振听]

48[50.64] 切5饼 => 24.25一向听 [12改良] [123456789p 56789s]

31[33.62] 77索碰,切5饼 => 5.48听牌数 [15 改良] [123456789p]

*/
// 何切分析結果を表示（1行）
func (r *analysisResult) printWaitsWithImproves13_oneRow() {
	discardTile34 := r.discardTile34
	openTiles34 := r.openTiles34
	result13 := r.result13

	shanten := result13.Shanten

	// 進張数
	waitsCount := result13.Waits.AllCount()
	c := getWaitsCountColor(shanten, float64(waitsCount))
	color.New(c).Printf("%2d", waitsCount)
	// 改良進張均値
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

	// 鳴き分析
	if discardTile34 != -1 {
		if len(openTiles34) > 0 {
			meldType := "チー"
			if openTiles34[0] == openTiles34[1] {
				meldType = "ポン"
			}
			color.New(color.FgHiWhite).Printf("%s%s", string([]rune(util.MahjongZH[openTiles34[0]])[:1]), util.MahjongZH[openTiles34[1]])
			fmt.Printf("%s,", meldType)
		}
		// 捨て牌
		if r.isDiscardTileDora {
			color.New(color.FgHiWhite).Print("ドラ")
		} else {
			fmt.Print("切")
		}
		tileZH := util.MahjongZH[discardTile34]
		if discardTile34 >= 27 {
			tileZH = " " + tileZH
		}
		if r.mixedRiskTable != nil {
			// 実際の危険度に基づいて捨て牌の危険度を表示
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
		// 次の進張数の平均
		incShanten := shanten - 1
		c := getWaitsCountColor(incShanten, result13.AvgNextShantenWaitsCount)
		color.New(c).Printf("%5.2f", result13.AvgNextShantenWaitsCount)
		fmt.Printf("%s", util.NumberToChineseShanten(incShanten))
		if incShanten >= 1 {
			//fmt.Printf("進張")
		} else { // incShanten == 0
			fmt.Printf("数")
			//if showAgariAboveShanten1 {
			//	fmt.Printf("（%.2f%% 参考和率）", result13.AvgAgariRate)
			//}
		}
	} else { // shanten == 0
		// 和了率
		// 振聴や片聴の場合は赤で表示
		if result13.FuritenRate == 1 || result13.IsPartWait {
			color.New(color.FgHiRed).Printf("%5.2f%% 参考和率", result13.AvgAgariRate)
		} else {
			fmt.Printf("%5.2f%% 参考和率", result13.AvgAgariRate)
		}
	}

	// 手牌速度、素早く局を進めるため
	if result13.MixedWaitsScore > 0 && shanten >= 1 && shanten <= 2 {
		fmt.Print(" ")
		if r.highlightMixedScore {
			color.New(color.FgHiWhite).Printf("[%5.2f速度]", result13.MixedWaitsScore)
		} else {
			fmt.Printf("[%5.2f速度]", result13.MixedWaitsScore)
		}
	}

	// 局収支
	if showScore && result13.MixedRoundPoint != 0.0 {
		fmt.Print(" ")
		color.New(color.FgHiGreen).Printf("[局収支%4d]", int(math.Round(result13.MixedRoundPoint)))
	}

	// (ツモ)栄和点数
	if result13.DamaPoint > 0 {
		fmt.Print(" ")
		ronType := "栄和"
		if !result13.IsNaki {
			ronType = "ツモ"
		}
		color.New(color.FgHiGreen).Printf("[%s%d]", ronType, int(math.Round(result13.DamaPoint)))
	}

	// リーチ点数、自摸、一発、裏ドラを考慮
	if result13.RiichiPoint > 0 {
		fmt.Print(" ")
		color.New(color.FgHiGreen).Printf("[リーチ%d]", int(math.Round(result13.RiichiPoint)))
	}

	if len(result13.YakuTypes) > 0 {
		// 役種（2向聴以内で表示）
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
				// デバッグ
				fmt.Print(" ")
				color.New(color.FgHiGreen).Printf(util.YakuTypesWithDoraToStr(result13.YakuTypes, result13.DoraCount))
			}
			// 片面待ち
			if result13.IsPartWait {
				fmt.Print(" ")
				color.New(color.FgHiRed).Printf("[片面待ち]")
			}
		}
	} else if result13.IsNaki && shanten >= 0 && shanten <= 2 {
		// 鳴き時の無役提示（聴牌から2向聴まで）
		fmt.Print(" ")
		color.New(color.FgHiRed).Printf("[無役]")
	}

	// フリテン表示
	if result13.FuritenRate > 0 {
		fmt.Print(" ")
		if result13.FuritenRate < 1 {
			color.New(color.FgHiYellow).Printf("[フリテンの可能性]")
		} else {
			color.New(color.FgHiRed).Printf("[フリテン]")
		}
	}

	// 改良数
	if showScore {
		fmt.Print(" ")
		if len(result13.Improves) > 0 {
			fmt.Printf("[%2d改良]", len(result13.Improves))
		} else {
			fmt.Print(strings.Repeat(" ", 4))
			fmt.Print(strings.Repeat("　", 2)) // 全角空白
		}
	}

	// 待ち牌タイプ
	fmt.Print(" ")
	waitTiles := result13.Waits.AvailableTiles()
	fmt.Print(util.TilesToStrWithBracket(waitTiles))

	//

	fmt.Println()

	if showImproveDetail {
		for tile, waits := range result13.Improves {
			fmt.Printf("ツモ %s で改良 %s\n", util.Mahjong[tile], waits.String())
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
		fmt.Print("鳴き後")
	}
	fmt.Println(util.NumberToChineseShanten(results14[0].Result13.Shanten) + "：")

	if results14[0].Result13.Shanten == 0 {
		// 聴牌が同じだが打点が異なるかどうかを確認
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
			color.HiGreen("注意: 打点が異なる聴牌選択があります")
		}
	}

	// FIXME: 選択肢が多い場合、何切選択を簡略化する方法？
	//const maxShown = 10
	//if len(results14) > maxShown { // 出力数を制限
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
