package monitor

import (
	"context"
	"cy-platforms-status-monitor/internal/snapshot"
	"time"
)

func Aggregator(ctx context.Context,resCh <- chan CheckResult,eventsCh chan <- Event) {
	state := make(map[string]*State)

	for {
		select {
		case <- ctx.Done(): return
		case res,ok := <-resCh:
			//graceful exit
			if !ok {
				return
			}

			st,ok := state[res.TargetName]
			if !ok {
				state[res.TargetName] = &State{}
				st = state[res.TargetName]
			}

			prevUp := st.LastUp
			updateState(st,res)
			if st.TotalChecks > 1 && prevUp != res.Up {
				event := Event{
					TargetName: res.TargetName,
					URL: res.URL,
					From: prevUp,
					To: res.Up,
					At: res.At,
				}
				//push to events
				select {
				case eventsCh<- event: 
				case <-ctx.Done():

				}
			}
			//build snapshot
			snapshot.Publish(buildSnapshot(state))
		}
	}
}

func buildSnapshot(states map[string]*State) snapshot.Snapshot {
	all := make([]snapshot.StateDTO, 0, len(states))
	byName := make(map[string]snapshot.StateDTO, len(states))

	for _, st := range states {
		dto := snapshot.StateDTO{
			Name:        st.Name,
			URL:         st.URL,
			Up:          st.LastUp,
			LastChecked: st.LastChecked.UTC().Format(time.RFC3339),
			LatencyMs:   st.LastLatency.Milliseconds(),
			StatusCode:  st.LastStatusCode,
			LastError:   st.LastError,

			ConsecutiveSuccess: st.ConsecutiveSuccess,
			ConsecutiveFail:    st.ConsecutiveFail,
			TotalChecks:        st.TotalChecks,
			TotalFails:         st.TotalFails,
		}

		all = append(all, dto)
		byName[dto.Name] = dto
	}

	return snapshot.Snapshot{
		All:    all,
		ByName: byName,
	}
}


func updateState(state *State, res CheckResult) {
	state.LastChecked = time.Now()
	state.LastUp = res.Up
	state.Name = res.TargetName
	state.LastLatency = res.Latency
	state.LastStatusCode = res.StatusCode
	state.URL = res.URL
	state.TotalChecks++

	if res.Up {
		state.ConsecutiveSuccess++
		state.ConsecutiveFail = 0
	} else {
		state.TotalFails++
		state.ConsecutiveFail++
		state.LastError = res.Error
	}


}