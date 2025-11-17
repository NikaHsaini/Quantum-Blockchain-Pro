from PySide6.QtWidgets import QWidget, QVBoxLayout, QLabel, QLineEdit, QGridLayout, QGroupBox, QTextEdit
from PySide6.QtCore import Qt

class StatusDisplay(QGroupBox):
    """Widget pour afficher les informations clés du node."""
    def __init__(self, title="Statut du Node", parent=None):
        super().__init__(title, parent)
        self.layout = QGridLayout()
        self.setLayout(self.layout)
        
        self.labels = {}
        self.fields = {}
        
        self._create_field("Hauteur de la Chaîne:", "chain_height", 0, 0)
        self._create_field("Supply Totale (QBTC):", "total_supply", 0, 2)
        self._create_field("Pairs Connectés:", "connected_peers", 1, 0)
        self._create_field("Statut du Node:", "is_running", 1, 2)
        self._create_field("Dernier Hash:", "latest_hash", 2, 0, colspan=4)
        
        self.log_area = QTextEdit()
        self.log_area.setReadOnly(True)
        self.log_area.setPlaceholderText("Logs de minage quantique...")
        self.layout.addWidget(QLabel("Logs de Minage Quantique:"), 3, 0, 1, 4)
        self.layout.addWidget(self.log_area, 4, 0, 1, 4)

    def _create_field(self, label_text, field_key, row, col, colspan=2):
        """Crée une paire label/champ de valeur dans la grille."""
        label = QLabel(label_text)
        field = QLineEdit("N/A")
        field.setReadOnly(True)
        field.setStyleSheet("background-color: #f0f0f0;")
        
        self.labels[field_key] = label
        self.fields[field_key] = field
        
        self.layout.addWidget(label, row, col)
        self.layout.addWidget(field, row, col + 1, 1, colspan - 1)

    def update_status(self, status_data):
        """Met à jour les champs d'affichage avec les données du statut."""
        self.fields["chain_height"].setText(str(status_data.get("chain_height", "N/A")))
        self.fields["total_supply"].setText(f"{status_data.get('total_supply', 'N/A'):.8f}")
        self.fields["connected_peers"].setText(str(status_data.get("connected_peers", "N/A")))
        
        is_running = status_data.get("is_running", False)
        status_text = "EN COURS" if is_running else "ARRÊTÉ"
        color = "green" if is_running else "red"
        self.fields["is_running"].setText(status_text)
        self.fields["is_running"].setStyleSheet(f"background-color: #f0f0f0; color: {color}; font-weight: bold;")
        
        latest_hash = status_data.get("latest_hash", "N/A")
        self.fields["latest_hash"].setText(latest_hash)
        
        # Mise à jour des logs (simulée)
        self.log_area.setText(status_data.get("mining_logs", "Logs de minage quantique..."))

class MinedBlocksDisplay(QGroupBox):
    """Widget pour afficher les blocs minés (simplifié pour le dernier bloc)."""
    def __init__(self, title="Dernier Bloc Miné", parent=None):
        super().__init__(title, parent)
        self.layout = QVBoxLayout()
        self.setLayout(self.layout)
        
        self.last_block_label = QLabel("Index: N/A\nHash: N/A")
        self.layout.addWidget(self.last_block_label)

    def update_last_block(self, block_index, block_hash):
        """Met à jour l'affichage du dernier bloc miné."""
        self.last_block_label.setText(f"Index: {block_index}\nHash: {block_hash[:64]}...") # Afficher les 64 premiers caractères du hash

# Les autres composants seront intégrés directement dans gui.py pour la simplicité.
