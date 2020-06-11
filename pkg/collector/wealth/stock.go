package wealth

type Option struct {
	StrikePrice      float64 `json:"strike-price"`
	ExerciseBuyPrice float64 `json:"exercise-buy-price"`
	Shares           float64 `json:"shares"`
	ExerciseDate     string  `json:"exercise-date"`
}

func (o *Option) GetExerciseTaxable() float64 {
	return o.ExerciseBuyPrice - o.StrikePrice
}

func (o *Option) GetExerciseTaxes() float64 {
	return o.Shares * o.GetExerciseTaxable() * 0.65 // 65%
}

func (o *Option) GetMarketValue(ddog float64) float64 {
	return o.Shares * ddog
}

func (o *Option) GetGainTaxable(ddog float64) float64 {
	return ddog - o.ExerciseBuyPrice
}

func (o *Option) GetGainTaxes(ddog float64) float64 {
	return o.Shares * o.GetGainTaxable(ddog) * 0.3 // 30% flat
}

func (o *Option) InThePocket(ddog float64) float64 {
	return o.GetMarketValue(ddog) - o.GetExerciseTaxes() - o.GetGainTaxes(ddog)
}
