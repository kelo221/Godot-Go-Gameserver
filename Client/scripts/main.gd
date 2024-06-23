extends Node3D
const PLAYER = preload("res://scenes/player.tscn")
@onready var players: Node3D = $Players

func _ready():
	Client.connect("new_puppet", new_puppet)
	var player_instance = PLAYER.instantiate()
	players.add_child(player_instance)

func new_puppet(puppet : PlayerProto.Player) -> void:
	var puppet_instance = PLAYER.instantiate()
	puppet_instance.local_player_data = puppet
	puppet_instance.is_puppet = true
	var new_position := puppet.get_pos()
	puppet_instance.position = Vector3(new_position[0].get_x(), new_position[0].get_y(),new_position[0].get_z())
	players.add_child(puppet_instance)
