class PeerManager:
    def __init__(self):
        # Liste des pairs connectés (format: (adresse, port, websocket_conn))
        self.connected_peers = {} # {address: connection_object}
        self.known_peers = set() # {address:port}

    def add_known_peer(self, address_port):
        """Ajoute un pair à la liste des pairs connus."""
        self.known_peers.add(address_port)

    def remove_known_peer(self, address_port):
        """Retire un pair de la liste des pairs connus."""
        self.known_peers.discard(address_port)

    def add_connected_peer(self, address, connection):
        """Ajoute une connexion active."""
        self.connected_peers[address] = connection

    def remove_connected_peer(self, address):
        """Retire une connexion active."""
        if address in self.connected_peers:
            del self.connected_peers[address]

    def get_connected_peers_addresses(self):
        """Retourne la liste des adresses des pairs connectés."""
        return list(self.connected_peers.keys())

    def get_known_peers(self):
        """Retourne la liste des pairs connus."""
        return list(self.known_peers)

    def get_peer_count(self):
        """Retourne le nombre de pairs connectés."""
        return len(self.connected_peers)

if __name__ == '__main__':
    pm = PeerManager()
    pm.add_known_peer("ws://127.0.0.1:8001")
    pm.add_known_peer("ws://127.0.0.1:8002")
    
    print(f"Pairs connus: {pm.get_known_peers()}")
    
    # Simulation d'une connexion
    pm.add_connected_peer("ws://127.0.0.1:8001", "conn_obj_1")
    
    print(f"Pairs connectés: {pm.get_connected_peers_addresses()}")
    print(f"Nombre de pairs connectés: {pm.get_peer_count()}")
