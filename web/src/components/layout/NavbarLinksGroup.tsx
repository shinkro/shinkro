import { useState } from 'react';
import { UnstyledButton, Group, ThemeIcon, Text, Box, rem } from '@mantine/core';
import { FaChevronRight } from 'react-icons/fa';
import { Link } from '@tanstack/react-router';
import classes from './NavbarLinksGroup.module.css';

interface LinksGroupProps {
  icon: React.ComponentType<{style?:React.CSSProperties}>;
  label: string;
  initiallyOpened?: boolean;
  links?: { label: string; link: string }[];
  link?: string;
}

export function LinksGroup({ icon: Icon, label, initiallyOpened, links, link }: LinksGroupProps) {
  const hasLinks = Array.isArray(links);
  const [opened, setOpened] = useState(initiallyOpened || false);
  const items = (hasLinks ? links : []).map((linkItem) => (
    <Link
      key={linkItem.label}
      to={linkItem.link}
      style={{ textDecoration: 'none', color: 'inherit' }}
    >
      {({ isActive }) => (
        <Text
          className={classes.link}
          data-active={isActive || undefined}
          fz="sm"
          fw={500}
          pl={rem(21)}
          py={rem(8)}
        >
          {linkItem.label}
        </Text>
      )}
    </Link>
  ));

  if (link) {
    return (
      <Link to={link} style={{ textDecoration: 'none', color: 'inherit' }}>
        {({ isActive }) => (
          <UnstyledButton className={classes.control} data-active={isActive || undefined}>
            <Group justify="space-between" gap={0}>
              <Box style={{ display: 'flex', alignItems: 'center' }}>
                <ThemeIcon variant="light" size={30}>
                  <Icon style={{ width: rem(18), height: rem(18) }} />
                </ThemeIcon>
                <Box ml="md">{label}</Box>
              </Box>
            </Group>
          </UnstyledButton>
        )}
      </Link>
    );
  }

  return (
    <>
      <UnstyledButton
        onClick={() => setOpened((o) => !o)}
        className={classes.control}
      >
        <Group justify="space-between" gap={0}>
          <Box style={{ display: 'flex', alignItems: 'center' }}>
            <ThemeIcon variant="light" size={30}>
              <Icon style={{ width: rem(18), height: rem(18) }} />
            </ThemeIcon>
            <Box ml="md">{label}</Box>
          </Box>
          {hasLinks && (
            <FaChevronRight
              className={classes.chevron}
              style={{
                width: rem(16),
                height: rem(16),
                transform: opened ? 'rotate(-90deg)' : 'none',
              }}
            />
          )}
        </Group>
      </UnstyledButton>
      {hasLinks ? <Box className={classes.collapse}>{items}</Box> : null}
    </>
  );
}
