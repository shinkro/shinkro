import { Link } from "@tanstack/react-router";
import { Group, ActionIcon, Button } from "@mantine/core";
import { FaEye } from "react-icons/fa";
import { ConfirmDeleteIcon } from "@components/alerts/ConfirmDeleteIcon";
import { InfoTooltip } from "./InfoTooltip";

interface ActionsCellProps {
    onView: () => void;
    inspectId?: number;
    onDelete?: () => void;
    deleteTitle?: string;
    deleteMessage?: string;
    isDeleting?: boolean;
}

export const ActionsCell = ({ 
    onView,
    inspectId,
    onDelete, 
    deleteTitle = "Delete",
    deleteMessage = "Are you sure you want to delete this item?",
    isDeleting = false
}: ActionsCellProps) => {
    return (
        <Group gap="xs" justify="center">
            {inspectId && <Button size="compact-sm" renderRoot={props=><Link {...props} to="/sync/events/$eventId" params={{eventId:String(inspectId)}}/>}>Inspect</Button>}
            <InfoTooltip label="View details">
                <ActionIcon variant="outline" onClick={onView}>
                    <FaEye size={16} />
                </ActionIcon>
            </InfoTooltip>
            {onDelete && (
                <ConfirmDeleteIcon
                    onConfirm={onDelete}
                    title={deleteTitle}
                    message={deleteMessage}
                    loading={isDeleting}
                    variant="outline"
                />
            )}
        </Group>
    );
};

