import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { EditorMode } from '@grafana/plugin-ui';
import { copyTextToClipboard } from '@grafana/ui';

import { applyQueryDefaults } from '../defaults';
import { QueryFormat } from '../types';

import { QueryHeader, type QueryHeaderProps } from './QueryHeader';
import { buildMockDB } from './SqlComponents.testHelpers';

jest.mock('@grafana/ui', () => ({
  ...jest.requireActual('@grafana/ui'),
  copyTextToClipboard: jest.fn(),
}));

const copyMock = jest.mocked(copyTextToClipboard);

function setup() {
  const onChange = jest.fn();
  const props: QueryHeaderProps = {
    db: { ...buildMockDB(), toRawSql: () => 'SELECT builder' },
    dialect: 'postgres',
    isQueryRunnable: true,
    onChange,
    onQueryRowChange: jest.fn(),
    onRunQuery: jest.fn(),
    preconfiguredDataset: 'db',
    query: applyQueryDefaults({
      refId: 'A',
      editorMode: EditorMode.Code,
      format: QueryFormat.Table,
      rawSql: 'SELECT edited',
    }),
    queryRowFilter: { filter: false, group: false, order: false, preview: false },
  };
  render(<QueryHeader {...props} />);
  return { onChange };
}

async function clickCopyAndSwitch() {
  await userEvent.click(screen.getByRole('radio', { name: 'Builder' }));
  await userEvent.click(await screen.findByRole('button', { name: 'Copy code and switch' }));
}

describe('QueryHeader copy and switch', () => {
  beforeEach(() => copyMock.mockReset());

  it('switches to builder after the copy succeeds', async () => {
    copyMock.mockResolvedValue();
    const { onChange } = setup();

    await clickCopyAndSwitch();

    expect(copyMock).toHaveBeenCalledWith('SELECT edited');
    await waitFor(() =>
      expect(onChange).toHaveBeenCalledWith(
        expect.objectContaining({ rawSql: 'SELECT builder', editorMode: EditorMode.Builder })
      )
    );
    expect(screen.queryByRole('button', { name: 'Copy code and switch' })).not.toBeInTheDocument();
  });

  it('keeps the code and the dialog open when the copy fails', async () => {
    copyMock.mockRejectedValue(new Error('refused'));
    const { onChange } = setup();

    await clickCopyAndSwitch();

    expect(await screen.findByText('Could not copy the code to the clipboard')).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Copy code and switch' })).toBeInTheDocument();
  });

  it('switches on retry once the copy succeeds', async () => {
    copyMock.mockRejectedValueOnce(new Error('refused')).mockResolvedValueOnce();
    const { onChange } = setup();

    await clickCopyAndSwitch();
    await screen.findByText('Could not copy the code to the clipboard');
    await userEvent.click(screen.getByRole('button', { name: 'Copy code and switch' }));

    await waitFor(() =>
      expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ editorMode: EditorMode.Builder }))
    );
  });

  it('clears the copy error when the dialog is reopened', async () => {
    copyMock.mockRejectedValue(new Error('refused'));
    setup();

    await clickCopyAndSwitch();
    await screen.findByText('Could not copy the code to the clipboard');
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    await userEvent.click(screen.getByRole('radio', { name: 'Builder' }));

    await screen.findByRole('button', { name: 'Copy code and switch' });
    expect(screen.queryByText('Could not copy the code to the clipboard')).not.toBeInTheDocument();
  });
});
