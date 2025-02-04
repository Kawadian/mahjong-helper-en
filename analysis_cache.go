package main

import (
	"fmt"

	"github.com/EndlessCheng/mahjong-helper/util"
	"github.com/fatih/color"
)

type analysisOpType int

const (
	analysisOpTypeTsumo     analysisOpType = iota
	analysisOpTypeChiPonKan                // Chi Pon Kan
	analysisOpTypeKan                      // Add Kan Ankan
)

// TODO: Remind "You should meld here, not skip"

type analysisCache struct {
	analysisOpType analysisOpType

	selfDiscardTile int
	//isSelfDiscardRedFive bool
	selfDiscardTileRisk float64
	isRiichiWhenDiscard bool
	meldType            int

	// What tile to meld with from the hand, empty means skip meld
	selfOpenTiles []int

	aiAttackDiscardTile      int
	aiDefenceDiscardTile     int
	aiAttackDiscardTileRisk  float64
	aiDefenceDiscardTileRisk float64

	tenpaiRate []float64 // TODO: Tenpai rate of three players
}

type roundAnalysisCache struct {
	isStart bool
	isEnd   bool
	cache   []*analysisCache

	analysisCacheBeforeChiPon *analysisCache
}

func (rc *roundAnalysisCache) print() {
	const (
		baseInfo  = "The assistant is calculating the recommended discard tile, please wait... (The result is for reference only)"
		emptyInfo = "--"
		sep       = "  "
	)

	done := rc != nil && rc.isEnd
	if !done {
		color.HiGreen(baseInfo)
	} else {
		// Check if the last one is Tsumo, if so, remove the recommendation
		if len(rc.cache) > 0 {
			latestCache := rc.cache[len(rc.cache)-1]
			if latestCache.selfDiscardTile == -1 {
				latestCache.aiAttackDiscardTile = -1
				latestCache.aiDefenceDiscardTile = -1
			}
		}
	}

	fmt.Print("Round ")
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

	fmt.Print("Self Discard")
	if done {
		for i, c := range rc.cache {
			suffix := ""
			if c.isRiichiWhenDiscard {
				suffix = "[Riichi]"
			} else if c.selfDiscardTile == -1 && i == len(rc.cache)-1 {
				//suffix = "[Tsumo]"
				// TODO: Draw
			}
			printTileInfo(c.selfDiscardTile, c.selfDiscardTileRisk, suffix)
		}
	}
	fmt.Println()

	fmt.Print("Attack Recommendation")
	if done {
		for _, c := range rc.cache {
			printTileInfo(c.aiAttackDiscardTile, c.aiAttackDiscardTileRisk, "")
		}
	}
	fmt.Println()

	fmt.Print("Defense Recommendation")
	if done {
		for _, c := range rc.cache {
			printTileInfo(c.aiDefenceDiscardTile, c.aiDefenceDiscardTileRisk, "")
		}
	}
	fmt.Println()

	fmt.Println()
}

// Actual discard tile (after drawing or melding)
func (rc *roundAnalysisCache) addSelfDiscardTile(tile int, risk float64, isRiichiWhenDiscard bool) {
	latestCache := rc.cache[len(rc.cache)-1]
	latestCache.selfDiscardTile = tile
	latestCache.selfDiscardTileRisk = risk
	latestCache.isRiichiWhenDiscard = isRiichiWhenDiscard
}

// Discard recommendation when drawing a tile
func (rc *roundAnalysisCache) addAIDiscardTileWhenDrawTile(attackTile int, defenceTile int, attackTileRisk float64, defenceDiscardTileRisk float64) {
	// Draw a tile, round +1
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

// Add Kan Ankan
func (rc *roundAnalysisCache) addKan(meldType int) {
	// latestCache is drawing a tile
	latestCache := rc.cache[len(rc.cache)-1]
	latestCache.analysisOpType = analysisOpTypeKan
	latestCache.meldType = meldType
	// After Kan, draw a tile again, round +1
}

// Chi Pon Kan
func (rc *roundAnalysisCache) addChiPonKan(meldType int) {
	if meldType == meldTypeMinkan {
		// Temporarily ignore Minkan, round +1 will be handled when drawing a tile
		return
	}
	// Round +1
	var newCache *analysisCache
	if rc.analysisCacheBeforeChiPon != nil {
		newCache = rc.analysisCacheBeforeChiPon // See addPossibleChiPonKan
		newCache.analysisOpType = analysisOpTypeChiPonKan
		newCache.meldType = meldType
		rc.analysisCacheBeforeChiPon = nil
	} else {
		// This code should not be triggered
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

// Chi Pon Kan Skip
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
	// Round number, Honba number
	wholeGameCache [][]*roundAnalysisCache

	majsoulRecordUUID string

	selfSeat int
}

func newGameAnalysisCache(majsoulRecordUUID string, selfSeat int) *gameAnalysisCache {
	cache := make([][]*roundAnalysisCache, 3*4) // Up to West 4
	for i := range cache {
		cache[i] = make([]*roundAnalysisCache, 100) // Up to 100 consecutive wins
	}
	return &gameAnalysisCache{
		wholeGameCache:    cache,
		majsoulRecordUUID: majsoulRecordUUID,
		selfSeat:          selfSeat,
	}
}

//

// TODO: Refactor into struct
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
	// Get the round and honba from the first action
	if len(actions) == 0 {
		return fmt.Errorf("Data error: This round data is empty")
	}

	newRoundAction := actions[0]
	data := newRoundAction.Action
	roundNumber := 4*(*data.Chang) + *data.Ju
	ben := *data.Ben
	roundCache := c.wholeGameCache[roundNumber][ben] // TODO: Suggest using atomic operations
	if roundCache == nil {
		roundCache = &roundAnalysisCache{isStart: true}
		if debugMode {
			fmt.Println("The assistant is calculating the recommended discard tile... Creating roundCache")
		}
		c.wholeGameCache[roundNumber][ben] = roundCache
	} else if roundCache.isStart {
		if debugMode {
			fmt.Println("No need to recalculate")
		}
		return nil
	}

	// Traverse self discard tiles, find the operation before discard
	// If it is a draw operation, calculate the AI attack discard and defense discard at this time
	// If it is a meld operation, calculate the AI attack discard at this time (set to -1 if no attack discard), set defense discard to -1
	// TODO: Player skips, but AI thinks it should meld?
	majsoulRoundData := &majsoulRoundData{selfSeat: c.selfSeat} // Note that a new majsoulRoundData is used here for calculation, there will be no data conflict
	majsoulRoundData.roundData = newGame(majsoulRoundData)
	majsoulRoundData.roundData.gameMode = gameModeRecordCache
	majsoulRoundData.skipOutput = true
	for i, action := range actions[:len(actions)-1] {
		if c.majsoulRecordUUID != getMajsoulCurrentRecordUUID() {
			if debugMode {
				fmt.Println("User exited this record")
			}
			// Exit early to reduce unnecessary calculations
			return nil
		}
		if debugMode {
			fmt.Println("The assistant is calculating the recommended discard tile... action", i)
		}
		majsoulRoundData.msg = action.Action
		majsoulRoundData.analysis()
	}
	roundCache.isEnd = true

	if c.majsoulRecordUUID != getMajsoulCurrentRecordUUID() {
		if debugMode {
			fmt.Println("User exited this record")
		}
		return nil
	}

	clearConsole()
	roundCache.print()

	return nil
}
