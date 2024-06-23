extends Node3D

@onready var collision_area: Area3D = $CollisionArea

# Called when the node enters the scene tree for the first time.
func _ready() -> void:
	await get_tree().create_timer(0.1).timeout
	collision_area.set_deferred("monitoring", true)


func _physics_process(delta: float) -> void:
	position -= self.transform.basis.z * delta * 20


func _on_kill_timer_timeout() -> void:
	queue_free()


func _on_area_3d_body_entered(body: Node3D) -> void:
	
	if get_meta("casted_by_puppet") == false:
		var damage_object := PlayerProto.Damage.new()
		damage_object.set_damage(25.0)
		damage_object.set_target_id(body.get_id())
		damage_object.set_caster_id(int(get_meta("Owner")))
		Client.send(4,damage_object.to_bytes())
		queue_free()
	else:
		queue_free()
