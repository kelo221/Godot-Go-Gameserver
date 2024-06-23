extends Control

@onready var player_data_container: VBoxContainer = $CenterContainer/Panel/MarginContainer/Scoreboard/PanelContainer/MarginContainer/PlayerDataContainer
const SCORE = preload("res://scenes/score.tscn")

func sort_by_score(a :ScoreboardProto.Score , b :ScoreboardProto.Score):
	if a.get_score() > b.get_score():
		return true
	return false

func update_scoreboard() -> void:
	
	for n in player_data_container.get_children():
			player_data_container.remove_child(n)
			n.queue_free()
	
	var scores = Client.scoreboard_data.get_score()
	scores.sort_custom(sort_by_score)
	for score in scores:
		var score_instance = SCORE.instantiate()
		score_instance.set_meta("who", score.get_name())
		score_instance.set_meta("score", str(score.get_score()))
		player_data_container.add_child(score_instance)
		
# Called when the node enters the scene tree for the first time.
func _ready() -> void:
	Client.connect("update_scoreboard", update_scoreboard)
