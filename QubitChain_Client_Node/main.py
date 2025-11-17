import os
import sys
import subprocess
import types

# Répertoire racine du projet (là où se trouve main.py)
PROJECT_ROOT = os.path.dirname(os.path.abspath(__file__))
VENV_PATH = os.path.join(PROJECT_ROOT, ".venv")

if os.name == "nt":
    VENV_PYTHON = os.path.join(VENV_PATH, "Scripts", "python.exe")
else:
    VENV_PYTHON = os.path.join(VENV_PATH, "bin", "python")


def in_venv() -> bool:
    """Retourne True si le script tourne déjà dans un venv."""
    return sys.prefix != getattr(sys, "base_prefix", sys.prefix)


def create_venv():
    """Crée l'environnement virtuel .venv si nécessaire."""
    if not os.path.isdir(VENV_PATH):
        print(f"[SETUP] Création de l'environnement virtuel dans {VENV_PATH}...")
        subprocess.check_call([sys.executable, "-m", "venv", VENV_PATH])
    else:
        print(f"[SETUP] Environnement virtuel déjà présent : {VENV_PATH}")


def install_dependencies():
    """Installe les dépendances dans le venv (.venv)."""
    req_file = os.path.join(PROJECT_ROOT, "requirements.txt")

    print("[SETUP] Mise à jour de pip dans le venv...")
    subprocess.check_call([VENV_PYTHON, "-m", "pip", "install", "--upgrade", "pip"])

    if os.path.isfile(req_file):
        print(f"[SETUP] Installation des dépendances depuis {req_file}...")
        subprocess.check_call([VENV_PYTHON, "-m", "pip", "install", "-r", req_file])
    else:
        print("[WARN] Aucun requirements.txt trouvé, saut de cette étape.")

    # Sécurité : s’assurer que PySide6 est bien présent
    try:
        subprocess.check_call([VENV_PYTHON, "-c", "import PySide6"])
        print("[SETUP] PySide6 déjà installé dans le venv.")
    except subprocess.CalledProcessError:
        print("[SETUP] Installation de PySide6 dans le venv...")
        subprocess.check_call([VENV_PYTHON, "-m", "pip", "install", "PySide6"])


def relaunch_in_venv():
    """Relance ce script avec le Python du venv."""
    print(f"[SETUP] Relance de main.py avec le Python du venv : {VENV_PYTHON}")
    os.execv(VENV_PYTHON, [VENV_PYTHON, os.path.join(PROJECT_ROOT, "main.py")])


def setup_alias_qubitchain_package():
    """
    Crée dynamiquement un faux package 'QubitChain' pointant sur le dossier
    QubitChain_Client_Node, pour que les imports du type
    'from QubitChain.node.node import QubitNode' fonctionnent.
    """
    if PROJECT_ROOT not in sys.path:
        sys.path.insert(0, PROJECT_ROOT)

    if "QubitChain" not in sys.modules:
        pkg = types.ModuleType("QubitChain")
        # Le package 'QubitChain' pointera vers PROJECT_ROOT
        pkg.__path__ = [PROJECT_ROOT]
        sys.modules["QubitChain"] = pkg


def run_gui():
    """Importe et lance la GUI une fois que tout est prêt."""
    setup_alias_qubitchain_package()

    try:
        from gui.gui import main as gui_main
    except ImportError as e:
        print("[ERROR] Impossible d'importer gui.gui.main :")
        print(e)
        sys.exit(1)

    print("[RUN] Lancement de la GUI QubitChain Client Node...")
    gui_main()


def main():
    if not in_venv():
        create_venv()
        install_dependencies()
        relaunch_in_venv()
    else:
        run_gui()


if __name__ == "__main__":
    main()


