import {Alert, Button, Group, Modal, ActionIcon} from "@mantine/core";
import {HiExclamationTriangle} from "react-icons/hi2";
import {useDisclosure} from "@mantine/hooks";
import {FaTrash} from "react-icons/fa";

interface ConfirmDeleteIconProps {
    onConfirm: () => void;
    title?: string;
    message?: string;
    confirmText?: string;
    cancelText?: string;
    loading?: boolean;
    icon?: React.ReactNode;
    color?: string;
    variant?: string;
}

export const ConfirmDeleteIcon = ({
    onConfirm,
    title = "Confirm Deletion",
    message = "",
    confirmText = "DELETE",
    cancelText = "CANCEL",
    loading = false,
    icon = <FaTrash size={18} />,
    color = "red",
    variant = "outline",
}: ConfirmDeleteIconProps) => {
    const [opened, {open, close}] = useDisclosure(false);

    return (
        <>
            <Modal opened={opened} onClose={close} title={title}>
                <Alert
                    icon={<HiExclamationTriangle size="1.5rem"/>}
                    title="Warning"
                    color="red"
                    variant="outline"
                    mb="md"
                >
                    This action cannot be undone! {message}
                </Alert>

                <Group justify="flex-end" mt="md">
                    <Button onClick={close}>
                        {cancelText}
                    </Button>
                    <Button
                        color="red"
                        loading={loading}
                        onClick={() => {
                            onConfirm();
                            close();
                        }}
                    >
                        {confirmText}
                    </Button>
                </Group>
            </Modal>

            <ActionIcon 
                maw="20px"
                variant={variant}
                color={color}
                onClick={open}
            >
                {icon}
            </ActionIcon>
        </>
    );
}; 