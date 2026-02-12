package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type Spot struct {
    ID       string `json:"id"`
    Number   int    `json:"number"`
    Occupied bool   `json:"occupied"`
    LastOpen string `json:"lastOpen,omitempty"`
}

type Park struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Spots []Spot `json:"spots"`
}

func fetchParks(base string) ([]Park, error) {
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Get(base + "/api/parks")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var parks []Park
    if err := json.NewDecoder(resp.Body).Decode(&parks); err != nil {
        return nil, err
    }
    return parks, nil
}

func postJSON(client *http.Client, url string, body string) (*http.Response, error) {
    req, err := http.NewRequest("POST", url, strings.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    return client.Do(req)
}

func printParks(parks []Park) {
    fmt.Println("\n=== Parks ===")
    for pi, p := range parks {
        fmt.Printf("[%d] %s (%s)\n", pi+1, p.Name, p.ID)
        for si, s := range p.Spots {
            occ := "EMPTY"
            if s.Occupied {
                occ = "OCCUPIED"
            }
            fmt.Printf("   (%d) Spot %d - %s - lastOpen:%s - id:%s\n", si+1, s.Number, occ, s.LastOpen, s.ID)
        }
    }
}

func main() {
    base := flag.String("base", "http://localhost:8080", "backend base URL")
    flag.Parse()

    reader := bufio.NewReader(os.Stdin)
    client := &http.Client{Timeout: 5 * time.Second}

    for {
        parks, err := fetchParks(*base)
        if err != nil {
            fmt.Println("Error fetching parks:", err)
        } else {
            printParks(parks)
        }

        fmt.Println("\nCommands:")
        fmt.Println("  r                - refresh")
        fmt.Println("  o P S            - open gate for Park P spot S (by index)")
        fmt.Println("  u P S V          - update occupancy for Park P spot S to V (0 empty,1 occupied)")
        fmt.Println("  q                - quit")
        fmt.Print("Enter command: ")

        line, _ := reader.ReadString('\n')
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        parts := strings.Fields(line)
        cmd := strings.ToLower(parts[0])
        switch cmd {
        case "q", "quit":
            fmt.Println("bye")
            return
        case "r", "refresh":
            // loop will refresh
            continue
        case "o":
            if len(parts) < 3 {
                fmt.Println("usage: o P S")
                continue
            }
            pidx := atoi(parts[1]) - 1
            sidx := atoi(parts[2]) - 1
            if pidx < 0 || pidx >= len(parks) {
                fmt.Println("invalid park index")
                continue
            }
            if sidx < 0 || sidx >= len(parks[pidx].Spots) {
                fmt.Println("invalid spot index")
                continue
            }
            parkID := parks[pidx].ID
            spotID := parks[pidx].Spots[sidx].ID
            url := *base + "/api/parks/" + parkID + "/spots/" + spotID + "/opengate"
            resp, err := postJSON(client, url, "{}")
            if err != nil {
                fmt.Println("request error:", err)
                continue
            }
            resp.Body.Close()
            fmt.Println("opengate status:", resp.Status)
        case "u":
            if len(parts) < 4 {
                fmt.Println("usage: u P S V")
                continue
            }
            pidx := atoi(parts[1]) - 1
            sidx := atoi(parts[2]) - 1
            v := parts[3]
            val := "false"
            if v == "1" || strings.ToLower(v) == "true" { val = "true" }
            if pidx < 0 || pidx >= len(parks) {
                fmt.Println("invalid park index")
                continue
            }
            if sidx < 0 || sidx >= len(parks[pidx].Spots) {
                fmt.Println("invalid spot index")
                continue
            }
            parkID := parks[pidx].ID
            spotID := parks[pidx].Spots[sidx].ID
            url := *base + "/api/parks/" + parkID + "/spots/" + spotID + "/update"
            body := "{\"occupied\":" + val + "}"
            resp, err := postJSON(client, url, body)
            if err != nil {
                fmt.Println("request error:", err)
                continue
            }
            resp.Body.Close()
            fmt.Println("update status:", resp.Status)
        default:
            fmt.Println("unknown command")
        }
        // small pause before next refresh
        time.Sleep(200 * time.Millisecond)
    }
}

func atoi(s string) int {
    var n int
    fmt.Sscan(s, &n)
    return n
}
