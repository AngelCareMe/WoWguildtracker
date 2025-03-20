package models

type Character struct {
	Name          string
	Realm         string
	Level         int
	PlayableClass string
	Guild         string
	MythicScore   float64
	Role          string
	Spec          string
}

type AccountCharacters struct {
	Characters []Character
}
