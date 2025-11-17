import json

# Définition des types de messages
MSG_TYPE = {
    "HANDSHAKE": "HANDSHAKE",
    "NEW_BLOCK": "NEW_BLOCK",
    "GET_CHAIN": "GET_CHAIN",
    "CHAIN_RESPONSE": "CHAIN_RESPONSE",
    "GET_PEERS": "GET_PEERS",
    "PEERS_RESPONSE": "PEERS_RESPONSE",
    "PING": "PING",
    "PONG": "PONG"
}

class Message:
    def __init__(self, type, payload=None):
        self.type = type
        self.payload = payload if payload is not None else {}

    def to_json(self):
        """Sérialise le message en chaîne JSON."""
        return json.dumps({"type": self.type, "payload": self.payload})

    @classmethod
    def from_json(cls, json_string):
        """Désérialise une chaîne JSON en objet Message."""
        try:
            data = json.loads(json_string)
            return cls(data.get("type"), data.get("payload"))
        except json.JSONDecodeError:
            return cls("ERROR", {"error": "Invalid JSON format"})

# Fonctions utilitaires pour créer des messages spécifiques
def create_new_block_message(block_data):
    return Message(MSG_TYPE["NEW_BLOCK"], {"block": block_data})

def create_peers_response_message(peers_list):
    return Message(MSG_TYPE["PEERS_RESPONSE"], {"peers": peers_list})

def create_handshake_message(node_id, chain_height):
    return Message(MSG_TYPE["HANDSHAKE"], {"node_id": node_id, "height": chain_height})

if __name__ == '__main__':
    # Exemple d'utilisation
    msg = create_new_block_message({"index": 1, "hash": "abc"})
    json_msg = msg.to_json()
    print(f"Message JSON: {json_msg}")
    
    re_msg = Message.from_json(json_msg)
    print(f"Message désérialisé: Type={re_msg.type}, Payload={re_msg.payload}")
