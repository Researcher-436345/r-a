"""add annotations

Revision ID: 3a1c8f9e2b4d
Revises: 2e7d61697bfd
Create Date: 2026-07-15 20:30:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

# revision identifiers, used by Alembic.
revision: str = '3a1c8f9e2b4d'
down_revision: Union[str, Sequence[str], None] = '2e7d61697bfd'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'annotations',
        sa.Column('id', sa.UUID(), nullable=False),
        sa.Column('paper_id', sa.UUID(), nullable=False),
        sa.Column('user_id', sa.UUID(), nullable=False),
        sa.Column('page', sa.Integer(), nullable=False),
        sa.Column('rect', postgresql.JSONB(astext_type=sa.Text()), nullable=True),
        sa.Column('selected_text', sa.Text(), nullable=False),
        sa.Column('note', sa.Text(), nullable=False),
        sa.Column('color', sa.String(length=16), nullable=False),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
        sa.Column('updated_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
        sa.ForeignKeyConstraint(['paper_id'], ['papers.id'], ondelete='CASCADE'),
        sa.ForeignKeyConstraint(['user_id'], ['users.id'], ondelete='CASCADE'),
        sa.PrimaryKeyConstraint('id'),
    )
    op.create_index(op.f('ix_annotations_paper_id'), 'annotations', ['paper_id'], unique=False)
    op.create_index(op.f('ix_annotations_user_id'), 'annotations', ['user_id'], unique=False)


def downgrade() -> None:
    op.drop_index(op.f('ix_annotations_user_id'), table_name='annotations')
    op.drop_index(op.f('ix_annotations_paper_id'), table_name='annotations')
    op.drop_table('annotations')
