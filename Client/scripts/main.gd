extends Node3D
const PLAYER = preload("res://scenes/player.tscn")
@onready var players: Node3D = $Players

func _ready():
	Client.connect("new_puppet", new_puppet)
	var player_instance = PLAYER.instantiate()
	players.add_child(player_instance)

func new_puppet(_position :Vector3, puppet : Protobuff.Player) -> void:
	var puppet_instance = PLAYER.instantiate()
	puppet_instance.local_player_data = puppet
	players.add_child(puppet_instance)
