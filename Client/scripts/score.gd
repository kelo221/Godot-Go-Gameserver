extends HSplitContainer

@onready var who: Label = $Who
@onready var score: Label = $Score


# Called when the node enters the scene tree for the first time.
func _ready() -> void:
	who.text = get_meta("who")
	score.text = get_meta("score")


# Called every frame. 'delta' is the elapsed time since the previous frame.
func _process(delta: float) -> void:
	pass
