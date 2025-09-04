// battleship v5
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type Ship interface {
	Position() (row, col int)
	Alive() bool
	TakeHit() bool
}

type BasicShip struct {
	Row int
	Col int
	hp  int
}

func NewBasicShip(row, col int) *BasicShip {
	return &BasicShip{Row: row, Col: col, hp: 1}
}

func (s *BasicShip) Position() (int, int) { return s.Row, s.Col }
func (s *BasicShip) Alive() bool          { return s.hp > 0 }
func (s *BasicShip) TakeHit() bool {
	if s.hp <= 0 {
		return true
	}
	s.hp--
	return s.hp <= 0
}

type ArmoredShip struct {
	Row   int
	Col   int
	Armor int
}

func NewArmoredShip(row, col, armor int) *ArmoredShip {
	return &ArmoredShip{Row: row, Col: col, Armor: armor}
}

func (s *ArmoredShip) Position() (int, int) { return s.Row, s.Col }
func (s *ArmoredShip) Alive() bool          { return s.Armor > 0 }
func (s *ArmoredShip) TakeHit() bool {
	if s.Armor <= 0 {
		return true
	}
	s.Armor--
	return s.Armor <= 0
}

type Board struct {
	Size  int
	Cells [][]rune
}

func NewBoard(size int) *Board {
	b := &Board{Size: size}
	b.Cells = make([][]rune, size)
	for i := 0; i < size; i++ {
		b.Cells[i] = make([]rune, size)
		for j := 0; j < size; j++ {
			b.Cells[i][j] = '.'
		}
	}
	return b
}

func (b *Board) InBounds(r, c int) bool {
	return r >= 0 && r < b.Size && c >= 0 && c < b.Size
}

func (b *Board) String() string {
	out := ""
	for i := 0; i < b.Size; i++ {
		for j := 0; j < b.Size; j++ {
			out += fmt.Sprintf("%c ", b.Cells[i][j])
		}
	}
	return out
}

type Player struct {
	Name string
}

type Game struct {
	Board *Board
	Ship  Ship
	Over  bool
	Shots int
}

type ShotResult struct {
	Hit       bool `json:"hit"`
	Destroyed bool `json:"destroyed"`
}

func NewGame(boardSize int, ship Ship) *Game {
	return &Game{
		Board: NewBoard(boardSize),
		Ship:  ship,
	}
}

func (g *Game) hitAt(r, c int) bool {
	sr, sc := g.Ship.Position()
	return r == sr && c == sc && g.Ship.Alive()
}

func (g *Game) TakeShot(r, c int) (ShotResult, error) {
	if g.Over {
		return ShotResult{}, fmt.Errorf("game is allready done")
	}
	if !g.Board.InBounds(r, c) {
		return ShotResult{}, fmt.Errorf("out of range")
	}
	g.Shots++
	sr := ShotResult{Hit: false, Destroyed: false}

	shipR, shipC := g.Ship.Position()
	if r == shipR && c == shipC && g.Ship.Alive() {
		sr.Hit = true
		destroyed := g.Ship.TakeHit()
		sr.Destroyed = destroyed
		if destroyed {
			g.Board.Cells[r][c] = 'X'
			g.Over = true
		} else {
			g.Board.Cells[r][c] = 'H'
		}
	} else {
		if g.Board.Cells[r][c] == '.' {
			g.Board.Cells[r][c] = 'o'
		}
	}
	return sr, nil
}

func RunConsoleUI(g *Game) {
	for !g.Over {
		fmt.Print(g.Board.String())
		var r, c int
		if _, err := fmt.Scan(&r, &c); err != nil {
			fmt.Println("error: ", err)
			continue
		}
		res, err := g.TakeShot(r, c)
		if err != nil {
			fmt.Println("error: ", err)
			continue
		}
		switch {
		case res.Destroyed:
			fmt.Println("Ship destroyed")
		case res.Hit:
			fmt.Println("Hit, but ship is alive")
		default:
			fmt.Println("Miss")
		}
	}
	fmt.Println("Final board: ")
	fmt.Print(g.Board.String())
}

func runHTTP(g *Game) {
	mux := http.NewServeMux()
	mux.HandleFunc("/shot", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		rStr := q.Get("r")
		cStr := q.Get("c")
		if rStr == "" || cStr == "" {
			http.Error(w, "Need r and c", http.StatusBadRequest)
			return
		}
		ri, err1 := strconv.Atoi(rStr)
		ci, err2 := strconv.Atoi(cStr)
		if err1 != nil || err2 != nil {
			http.Error(w, "r and c must be numbers", http.StatusBadRequest)
			return
		}

		res, err := g.TakeShot(ri, ci)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := map[string]any{
			"hit":       res.Hit,
			"destroyed": res.Destroyed,
			"shots":     g.Shots,
			"board":     g.Board.String(),
			"over":      g.Over,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/board", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(g.Board.String()))
	})

	addr := ":8080"
	log.Fatal(http.ListenAndServe(addr, mux))
}

func main() {
	httpMode := flag.Bool("http", false, "http ui mode")
	armor := flag.Int("armor", 0, "armor count")
	size := flag.Int("size", 5, "board size")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	row := rand.Intn(*size)
	col := rand.Intn(*size)

	var ship Ship
	if *armor > 0 {
		ship = NewArmoredShip(row, col, *armor)
	} else {
		ship = NewBasicShip(row, col)
	}

	game := NewGame(*size, ship)

	if *httpMode {
		runHTTP(game)
	} else {
		RunConsoleUI(game)
	}
}
