package main

import (
	"fmt"
	"github.com/EndlessCheng/mahjong-helper/util"
	"github.com/fatih/color"
)

type analysisOpType int

const (
	analysisOpTypeTsumo     analysisOpType = iota
	analysisOpTypeChiPonKan  // 吃 碰 明杠
	analysisOpTypeKan        // 加杠 暗杠
)

// TODO: 提醒「此处应该副露，不应跳过」

type analysisCache struct {
	analysisOpType analysisOpType

	selfDiscardTile int
	//isSelfDiscardRedFive bool
	selfDiscardTileRisk float64
	isRiichiWhenDiscard bool
	meldType            int

	// 用手牌中的什么牌去鸣牌，空就是跳过不鸣
	selfOpenTiles []int

	aiAttackDiscardTile      int
	aiDefenceDiscardTile     int
	aiAttackDiscardTileRisk  float64
	aiDefenceDiscardTileRisk float64

	tenpaiRate []float64 // TODO: 三家听牌率
}

type roundAnalysisCache struct {
	isStart bool
	isEnd   bool
	cache   []*analysisCache

	analysisCacheBeforeChiPon *analysisCache
}

func (rc *roundAnalysisCache) print() {
	const (
		baseInfo  = "アシスタントは推奨打牌を計算中です。しばらくお待ちください...（計算結果は参考用です）"
		emptyInfo = "--"
		sep       = "  "
	)

	done := rc != nil && rc.isEnd
	if !done {
		color.HiGreen(baseInfo)
	} else {
		// 检查最后的是否自摸，若为自摸则去掉推荐
		if len(rc.cache) > 0 {
			latestCache := rc.cache[len(rc.cache)-1]
			if latestCache.selfDiscardTile == -1 {
				latestCache.aiAttackDiscardTile = -1
				latestCache.aiDefenceDiscardTile = -1
			}
		}
	}

	fmt.Print("巡目　　")
	if done {
		for i := range rc.cache {
			fmt.Printf("%s%2d", sep, i+1)
		}
	}
	fmt.Println()

	printTileInfo := func(tile int, risk float64, suffix string) {
		info := emptyInfo
		if tile != -1 {
			info = util.Mahjong[tile]
		}
		fmt.Print(sep)
		if info == emptyInfo || risk < 5 {
			fmt.Print(info)
		} else {
			color.New(getNumRiskColor(risk)).Print(info)
		}
		fmt.Print(suffix)
	}
	fmt.Print("打牌　　　")
	if done {
		for i, c := range rc.cache {
			suffix := ""
			if c.isRiichiWhenDiscard {
				suffix = "[リーチ]"
			} else if c.selfDiscardTile == -1 && i == len(rc.cache)-1 {
				//suffix = "[ツモ]"
				// TODO: 流局
			}
			printTileInfo(c.selfDiscardTile, c.selfDiscardTileRisk, suffix)
		}
	}
	fmt.Println()

	fmt.Print("攻め推奨")
	if done {
		for _, c := range rc.cache {
			printTileInfo(c.aiAttackDiscardTile, c.aiAttackDiscardTileRisk, "")
		}
	}
	fmt.Println()

	fmt.Print("守り推奨")
	if done {
		for _, c := range rc.cache {
			printTileInfo(c.aiDefenceDiscardTile, c.aiDefenceDiscardTileRisk, "")
		}
	}
	fmt.Println()

	fmt.Println()
}

// （摸牌后、鸣牌后的）实际舍牌
func (rc *roundAnalysisCache) addSelfDiscardTile(tile int, risk float64, isRiichiWhenDiscard bool) {
	latestCache := rc.cache[len(rc.cache)-1]
	latestCache.selfDiscardTile = tile
	latestCache.selfDiscardTileRisk = risk
	latestCache.isRiichiWhenDiscard = isRiichiWhenDiscard
}

// 摸牌时的切牌推荐
func (rc *roundAnalysisCache) addAIDiscardTileWhenDrawTile(attackTile int, defenceTile int, attackTileRisk float64, defenceDiscardTileRisk float64) {
	// 摸牌，巡目+1
	rc.cache = append(rc.cache, &analysisCache{
		analysisOpType:           analysisOpTypeTsumo,
		selfDiscardTile:          -1,
		aiAttackDiscardTile:      attackTile,
		aiDefenceDiscardTile:     defenceTile,
		aiAttackDiscardTileRisk:  attackTileRisk,
		aiDefenceDiscardTileRisk: defenceDiscardTileRisk,
	})
	rc.analysisCacheBeforeChiPon = nil
}

// 加杠 暗杠
func (rc *roundAnalysisCache) addKan(meldType int) {
	// latestCache 是摸牌
	latestCache := rc.cache[len(rc.cache)-1]
	latestCache.analysisOpType = analysisOpTypeKan
	latestCache.meldType = meldType
	// 杠完之后又会摸牌，巡目+1
}

// 吃 碰 明杠
func (rc *roundAnalysisCache) addChiPonKan(meldType int) {
	if meldType == meldTypeMinkan {
		// 暂时忽略明杠，巡目不+1，留给摸牌时+1
		return
	}
	// 巡目+1
	var newCache *analysisCache
	if rc.analysisCacheBeforeChiPon != nil {
		newCache = rc.analysisCacheBeforeChiPon // 见 addPossibleChiPonKan
		newCache.analysisOpType = analysisOpTypeChiPonKan
		newCache.meldType = meldType
		rc.analysisCacheBeforeChiPon = nil
	} else {
		// 此处代码应该不会触发
		if debugMode {
			panic("rc.analysisCacheBeforeChiPon == nil")
		}
		newCache = &analysisCache{
			analysisOpType:       analysisOpTypeChiPonKan,
			selfDiscardTile:      -1,
			aiAttackDiscardTile:  -1,
			aiDefenceDiscardTile: -1,
			meldType:             meldType,
		}
	}
	rc.cache = append(rc.cache, newCache)
}

// 吃 碰 杠 跳过
func (rc *roundAnalysisCache) addPossibleChiPonKan(attackTile int, attackTileRisk float64) {
	rc.analysisCacheBeforeChiPon = &analysisCache{
		analysisOpType:          analysisOpTypeChiPonKan,
		selfDiscardTile:         -1,
		aiAttackDiscardTile:     attackTile,
		aiDefenceDiscardTile:    -1,
		aiAttackDiscardTileRisk: attackTileRisk,
	}
}

//

type gameAnalysisCache struct {
	// 局数 本场数
	wholeGameCache [][]*roundAnalysisCache

	majsoulRecordUUID string

	selfSeat int
}

func newGameAnalysisCache(majsoulRecordUUID string, selfSeat int) *gameAnalysisCache {
	cache := make([][]*roundAnalysisCache, 3*4) // 最多到西四
	for i := range cache {
		cache[i] = make([]*roundAnalysisCache, 100) // 最多连庄
	}
	return &gameAnalysisCache{
		wholeGameCache:    cache,
		majsoulRecordUUID: majsoulRecordUUID,
		selfSeat:          selfSeat,
	}
}

//

// TODO: 重构成 struct
var (
	_analysisCacheList = make([]*gameAnalysisCache, 4)
	_currentSeat       int
)

func resetAnalysisCache() {
	_analysisCacheList = make([]*gameAnalysisCache, 4)
}

func setAnalysisCache(analysisCache *gameAnalysisCache) {
	_analysisCacheList[analysisCache.selfSeat] = analysisCache
	_currentSeat = analysisCache.selfSeat
}

func getAnalysisCache(seat int) *gameAnalysisCache {
	if seat == -1 {
		return nil
	}
	return _analysisCacheList[seat]
}

func getCurrentAnalysisCache() *gameAnalysisCache {
	return getAnalysisCache(_currentSeat)
}
func (c *gameAnalysisCache) runMajsoulRecordAnalysisTask(actions majsoulRoundActions) error {
	// 最初のアクションから局と場を取得
	if len(actions) == 0 {
		return fmt.Errorf("データ異常：この局のデータが空です")
	}

	newRoundAction := actions[0]
	data := newRoundAction.Action
	roundNumber := 4*(*data.Chang) + *data.Ju
	ben := *data.Ben
	roundCache := c.wholeGameCache[roundNumber][ben] // TODO: アトミック操作を推奨
	if roundCache == nil {
		roundCache := &roundAnalysisCache{isStart: true}
		if debugMode {
			fmt.Println("アシスタントは推奨打牌を計算中です... roundCacheを作成")
		}
		c.wholeGameCache[roundNumber][ben] = roundCache
	} else if roundCache.isStart {
		if debugMode {
			fmt.Println("重複計算は不要です")
		}
		return nil
	}

	// 自分の打牌をループして、打牌前の操作を見つける
	// ツモの場合、AIの攻めと守りの推奨打牌を計算
	// 鳴きの場合、AIの攻めの推奨打牌を計算（攻めがない場合は-1）、守りは-1
	// TODO: プレイヤーがスキップしたが、AIが鳴くべきと判断した場合？
	majsoulRoundData := &majsoulRoundData{selfSeat: c.selfSeat} // 注意：新しいmajsoulRoundDataで計算するためデータ競合はない
	majsoulRoundData.roundData = newGame(majsoulRoundData)
	majsoulRoundData.roundData.gameMode = gameModeRecordCache
	majsoulRoundData.skipOutput = true
	for i, action := range actions[:len(actions)-1] {
		if c.majsoulRecordUUID != getMajsoulCurrentRecordUUID() {
			if debugMode {
				fmt.Println("ユーザーが牌譜を終了しました")
			}
			// 早期終了で不要な計算を避ける
			return nil
		}
		if debugMode {
			fmt.Println("アシスタントは推奨打牌を計算中です... action", i)
		}
		majsoulRoundData.msg = action.Action
		majsoulRoundData.analysis()
	}
	roundCache.isEnd = true

	if c.majsoulRecordUUID != getMajsoulCurrentRecordUUID() {
		if debugMode {
			fmt.Println("ユーザーが牌譜を終了しました")
		}
		return nil
	}

	clearConsole()
	roundCache.print()

	return nil
}
