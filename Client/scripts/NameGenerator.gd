extends Node3D

const NOUNS := ["Computer", "Mountain", "Ocean", "Book", "Music", "Television", "Apple", "City", "Car", "Space"];
const ADJECTIVES := ["Happy", "Sad", "Excited", "Angry", "Joyful", "Peaceful", "Grumpy", "Elated", "Nervous", "Relaxed"];

func generate_name() -> StringName:
	return NOUNS.pick_random() + " " + ADJECTIVES.pick_random()

