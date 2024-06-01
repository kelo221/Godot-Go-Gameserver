extends MultiplayerSpawner

const PLAYER = preload("res://scenes/player.tscn")

func _ready() -> void:
	Client.connect("player_id_received", _on_player_id_received)


func _on_player_id_received() -> void:
	print("Hi")


func _spawn_player(data):
	var player = PLAYER.instance()
	return player
