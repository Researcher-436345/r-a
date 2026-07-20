"""baseline — схема пустая, готовы к EPIC-02 (users)

Revision ID: 20260715_0001
Revises:
Create Date: 2026-07-15

"""

from typing import Sequence, Union

# revision identifiers, used by Alembic.
revision: str = "20260715_0001"
down_revision: Union[str, Sequence[str], None] = None
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # Инфраструктурный ревизионный якорь. Таблицы добавим в EPIC-02+.
    pass


def downgrade() -> None:
    pass
